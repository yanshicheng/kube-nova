package notification

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// emailClient 邮件告警客户端
// 支持通过 SMTP 协议发送告警邮件，支持多种加密方式
type emailClient struct {
	*baseClient
	config *EmailConfig
}

// NewEmailClient 创建邮件客户端
// config 必须包含有效的 Email 配置
func NewEmailClient(config Config) (FullChannel, error) {
	if config.Email == nil {
		return nil, errorx.Msg("邮件配置不能为空")
	}

	// 验证邮件配置
	if err := validateEmailConfig(config.Email); err != nil {
		return nil, errorx.Msg(fmt.Sprintf("邮件配置验证失败: %v", err))
	}

	// 自动优化配置
	optimizeEmailConfig(config.Email)

	bc := newBaseClient(config)
	client := &emailClient{
		baseClient: bc,
		config:     config.Email,
	}

	client.l.Infof("创建邮件告警客户端: %s | SMTP: %s:%d | TLS: %v | StartTLS: %v",
		config.UUID, config.Email.SMTPHost, config.Email.SMTPPort,
		config.Email.ShouldUseTLS(), config.Email.UseStartTLS)
	return client, nil
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本
func (c *emailClient) WithContext(ctx context.Context) Client {
	// 创建新的 baseClient 副本
	newBaseClient := c.baseClient.WithContext(ctx).(*baseClient)

	// 创建新的 emailClient 实例（共享 config）
	return &emailClient{
		baseClient: newBaseClient,
		config:     c.config,
	}
}

// SendPrometheusAlert 发送 Prometheus 告警邮件
func (c *emailClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errorx.Msg("客户端已关闭")
	}

	// 检查速率限制
	if err := c.checkRateLimit(); err != nil {
		c.recordFailure()
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	// 获取收件人列表
	to, cc, bcc := c.getRecipients(opts.Mentions)
	if len(to) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "收件人列表为空",
			Timestamp: time.Now(),
			Error:     errorx.Msg("收件人列表为空"),
		}, errorx.Msg("收件人列表为空")
	}

	// 使用格式化器构建 HTML 邮件
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)

	// 检测是否为聚合告警（同一项目的多条告警）
	var subject, body string
	if opts.Severity == "mixed" || len(alerts) > 1 {
		// 聚合告警，使用聚合格式化
		aggregatedGroup := buildAggregatedAlertGroupFromAlerts(alerts, opts.ProjectName)
		subject, body = formatter.FormatAggregatedAlertForEmail(aggregatedGroup)
	} else {
		// 单条告警，使用原有格式化
		subject, body = formatter.FormatHTMLForEmail(opts, alerts)
	}

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeEmail),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_prometheus_alert",
		TargetInfo:  fmt.Sprintf("收件人: %v, 告警数量: %d, 项目: %s", to, len(alerts), opts.ProjectName),
	}

	// 发送邮件（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendEmail(subject, body, to, cc, bcc)
		return sendErr
	})

	// 记录统计
	if err == nil && result != nil && result.Success {
		c.recordSuccess(time.Since(startTime))
		result.CostMs = time.Since(startTime).Milliseconds()
		// 记录 Prometheus 指标
		GlobalMetricsCollector.RecordEmailSent(c.config.SMTPHost, "success", "unknown", time.Since(startTime))
		GlobalMetricsCollector.RecordAlertSent("email", "success", "unknown", time.Since(startTime))
	} else {
		c.recordFailure()
		// 记录 Prometheus 指标
		GlobalMetricsCollector.RecordEmailSent(c.config.SMTPHost, "failure", "unknown", time.Since(startTime))
		GlobalMetricsCollector.RecordAlertSent("email", "failure", "unknown", time.Since(startTime))
	}

	return result, err
}

// SendNotification 发送通用通知邮件
func (c *emailClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errorx.Msg("客户端已关闭")
	}

	// 检查速率限制
	if err := c.checkRateLimit(); err != nil {
		c.recordFailure()
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	// 获取收件人列表
	to, cc, bcc := c.getRecipients(opts.Mentions)
	if len(to) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "收件人列表为空",
			Timestamp: time.Now(),
			Error:     errorx.Msg("收件人列表为空"),
		}, errorx.Msg("收件人列表为空")
	}

	// 使用格式化器构建邮件
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)
	subject, body := formatter.FormatNotificationForEmail(opts)

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeEmail),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_notification",
		TargetInfo:  fmt.Sprintf("收件人: %v, 标题: %s", to, opts.Title),
	}

	// 发送邮件（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendEmail(subject, body, to, cc, bcc)
		return sendErr
	})

	// 记录统计
	if err == nil && result != nil && result.Success {
		c.recordSuccess(time.Since(startTime))
		result.CostMs = time.Since(startTime).Milliseconds()
		// 记录 Prometheus 指标
		GlobalMetricsCollector.RecordEmailSent(c.config.SMTPHost, "success", "unknown", time.Since(startTime))
		GlobalMetricsCollector.RecordAlertSent("email", "success", "unknown", time.Since(startTime))
	} else {
		c.recordFailure()
		// 记录 Prometheus 指标
		GlobalMetricsCollector.RecordEmailSent(c.config.SMTPHost, "failure", "unknown", time.Since(startTime))
		GlobalMetricsCollector.RecordAlertSent("email", "failure", "unknown", time.Since(startTime))
	}

	return result, err
}

// getRecipients 从 MentionConfig 中获取收件人列表
func (c *emailClient) getRecipients(mentions *MentionConfig) (to, cc, bcc []string) {
	if mentions == nil {
		return nil, nil, nil
	}

	to = mentions.EmailTo
	cc = mentions.EmailCC
	bcc = mentions.EmailBCC

	return to, cc, bcc
}

// sendEmail 发送邮件的核心方法
// 根据配置选择不同的加密方式发送
func (c *emailClient) sendEmail(subject, body string, to, cc, bcc []string) (*SendResult, error) {
	// 构建发件人地址
	from := c.config.Username
	if c.config.From != "" {
		from = c.config.From
	}

	// 构建邮件消息
	msg := c.buildEmailMessage(from, to, cc, subject, body)

	// 合并所有收件人（包括抄送和密送）
	allRecipients := make([]string, 0, len(to)+len(cc)+len(bcc))
	allRecipients = append(allRecipients, to...)
	allRecipients = append(allRecipients, cc...)
	allRecipients = append(allRecipients, bcc...)

	// SMTP 服务器地址
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)

	// SMTP 认证信息
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.SMTPHost)

	var err error

	// 根据配置选择发送方式
	// 优先级: SSL/TLS (465端口) > STARTTLS (587端口) > 无加密 (25端口)
	if c.config.ShouldUseTLS() {
		// 使用 SSL/TLS 加密（通常用于 465 端口）
		err = c.sendWithTLS(addr, auth, allRecipients, msg)
	} else if c.config.UseStartTLS {
		// 使用 STARTTLS 加密（通常用于 587 端口）
		err = c.sendWithStartTLS(addr, auth, allRecipients, msg)
	} else {
		// 无加密（通常用于 25 端口）
		err = smtp.SendMail(addr, auth, c.config.Username, allRecipients, msg)
	}

	if err != nil {
		// 增强错误处理，提供更友好的错误信息
		enhancedErr := enhanceEmailError(err, c.config)
		return &SendResult{
			Success:   false,
			Message:   enhancedErr.Error(),
			Timestamp: time.Now(),
			Error:     enhancedErr,
		}, enhancedErr
	}

	c.l.Infof("[邮件] 发送成功 | UUID: %s | 收件人: %v", c.baseClient.GetUUID(), to)
	return &SendResult{
		Success:   true,
		Message:   fmt.Sprintf("邮件发送成功，收件人数: %d", len(allRecipients)),
		Timestamp: time.Now(),
	}, nil
}

// buildEmailMessage 构建符合 SMTP 协议的邮件消息
func (c *emailClient) buildEmailMessage(from string, to, cc []string, subject, body string) []byte {
	var msg strings.Builder

	// 邮件头
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ",")))
	if len(cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cc, ",")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	// 空行分隔邮件头和正文
	msg.WriteString("\r\n")
	// 邮件正文
	msg.WriteString(body)

	return []byte(msg.String())
}

// enhanceEmailError 增强邮件错误信息，提供更友好的错误提示
func enhanceEmailError(err error, config *EmailConfig) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// 163 邮箱常见错误
	if strings.Contains(config.SMTPHost, "163.com") {
		if strings.Contains(errMsg, "554") {
			return errorx.Msg("163邮箱认证失败 (错误码554)。请检查：\n" +
				"1. 用户名必须是完整邮箱地址（如：user@163.com）\n" +
				"2. 密码必须是授权码，不是登录密码\n" +
				"3. 获取授权码步骤：登录163邮箱 → 设置 → POP3/SMTP/IMAP → 开启SMTP服务 → 生成授权码\n" +
				"4. 授权码格式通常为16位字符（如：ABCD1234EFGH5678）\n" +
				"5. 确认SMTP配置：smtp.163.com:465 (SSL) 或 smtp.163.com:25 (无加密)\n" +
				"6. 如果仍然失败，请检查IP是否被163限制，可尝试更换网络\n" +
				"原始错误: " + errMsg)
		}
		if strings.Contains(errMsg, "535") {
			return errorx.Msg("163邮箱用户名或授权码错误 (错误码535)。请确认：\n" +
				"1. 用户名为完整邮箱地址（如：user@163.com）\n" +
				"2. 使用授权码而非登录密码\n" +
				"3. 授权码格式正确（通常为16位）\n" +
				"4. 授权码未过期（建议重新生成）\n" +
				"原始错误: " + errMsg)
		}
		if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "timeout") {
			return errorx.Msg("163邮箱连接失败。请检查：\n" +
				"1. 网络连接是否正常\n" +
				"2. 防火墙是否阻止SMTP连接\n" +
				"3. 尝试使用不同端口：465(SSL)、587(STARTTLS)、25(无加密)\n" +
				"4. 如果在公司网络，请联系网络管理员检查邮件端口限制\n" +
				"原始错误: " + errMsg)
		}
	}

	// QQ 邮箱常见错误
	if strings.Contains(config.SMTPHost, "qq.com") {
		if strings.Contains(errMsg, "535") {
			return errorx.Msg("QQ邮箱认证失败 (错误码535)。请检查：\n" +
				"1. 用户名是否为完整QQ邮箱地址\n" +
				"2. 密码是否为授权码\n" +
				"3. 是否已开启SMTP服务\n" +
				"原始错误: " + errMsg)
		}
	}

	// Gmail 常见错误
	if strings.Contains(config.SMTPHost, "gmail.com") {
		if strings.Contains(errMsg, "535") {
			return errorx.Msg("Gmail认证失败 (错误码535)。请检查：\n" +
				"1. 是否启用了两步验证\n" +
				"2. 是否使用应用专用密码\n" +
				"3. 是否允许不够安全的应用访问\n" +
				"原始错误: " + errMsg)
		}
	}

	// 通用 SMTP 错误
	if strings.Contains(errMsg, "connection refused") {
		return errorx.Msg("无法连接到SMTP服务器。请检查：\n" +
			"1. SMTP服务器地址是否正确\n" +
			"2. 端口号是否正确\n" +
			"3. 网络连接是否正常\n" +
			"4. 防火墙是否阻止了连接\n" +
			"原始错误: " + errMsg)
	}

	if strings.Contains(errMsg, "timeout") {
		return errorx.Msg("连接超时。请检查：\n" +
			"1. 网络连接是否稳定\n" +
			"2. SMTP服务器是否响应正常\n" +
			"3. 是否需要调整超时时间\n" +
			"原始错误: " + errMsg)
	}

	if strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "tls") {
		return errorx.Msg("TLS/SSL证书验证失败。请检查：\n" +
			"1. 服务器证书是否有效\n" +
			"2. 是否需要设置 InsecureSkipVerify=true（仅测试环境）\n" +
			"3. 系统时间是否正确\n" +
			"原始错误: " + errMsg)
	}

	if strings.Contains(errMsg, "550") {
		return errorx.Msg("邮件被拒绝 (错误码550)。可能原因：\n" +
			"1. 收件人邮箱地址无效\n" +
			"2. 发件人被列入黑名单\n" +
			"3. 邮件内容被识别为垃圾邮件\n" +
			"4. 发件人域名未配置SPF记录\n" +
			"原始错误: " + errMsg)
	}

	if strings.Contains(errMsg, "421") {
		return errorx.Msg("服务暂时不可用 (错误码421)。请稍后重试。\n" +
			"可能原因：\n" +
			"1. 服务器负载过高\n" +
			"2. 发送频率过快\n" +
			"3. IP被临时限制\n" +
			"原始错误: " + errMsg)
	}

	// 返回原始错误，但添加通用建议
	return errorx.Msg("邮件发送失败。建议检查：\n" +
		"1. SMTP配置是否正确\n" +
		"2. 网络连接是否正常\n" +
		"3. 认证信息是否有效\n" +
		"4. 收件人地址是否正确\n" +
		"原始错误: " + errMsg)
}

// sendWithTLS 使用 SSL/TLS 加密发送邮件
// 适用于 465 端口，建立连接时就使用 TLS
func (c *emailClient) sendWithTLS(addr string, auth smtp.Auth, recipients []string, msg []byte) error {
	// TLS 配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify, // 从配置读取是否跳过证书验证
		ServerName:         c.config.SMTPHost,
	}

	// 使用带超时的 TLS 连接
	dialer := &net.Dialer{
		Timeout: c.baseClient.config.Options.Timeout,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("TLS 连接失败: %v", err))
	}
	defer conn.Close()

	// 创建 SMTP 客户端
	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("创建 SMTP 客户端失败: %v", err))
	}
	defer client.Close()

	// SMTP 认证
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return errorx.Msg(fmt.Sprintf("SMTP 认证失败: %v", err))
		}
	}

	// 设置发件人
	if err := client.Mail(c.config.Username); err != nil {
		return errorx.Msg(fmt.Sprintf("设置发件人失败: %v", err))
	}

	// 设置收件人（包括所有 To、Cc、Bcc）
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return errorx.Msg(fmt.Sprintf("设置收件人 %s 失败: %v", recipient, err))
		}
	}

	// 写入邮件内容
	writer, err := client.Data()
	if err != nil {
		return errorx.Msg(fmt.Sprintf("获取数据写入器失败: %v", err))
	}

	if _, err := writer.Write(msg); err != nil {
		return errorx.Msg(fmt.Sprintf("写入邮件数据失败: %v", err))
	}

	if err := writer.Close(); err != nil {
		return errorx.Msg(fmt.Sprintf("关闭数据写入器失败: %v", err))
	}

	// 正常退出
	return client.Quit()
}

// sendWithStartTLS 使用 STARTTLS 加密发送邮件
// 适用于 587 端口，先建立明文连接再升级为 TLS
func (c *emailClient) sendWithStartTLS(addr string, auth smtp.Auth, recipients []string, msg []byte) error {
	// 使用带超时的连接
	dialer := &net.Dialer{
		Timeout: c.baseClient.config.Options.Timeout,
	}

	// 建立明文 TCP 连接
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("连接 SMTP 服务器失败: %v", err))
	}

	// 使用标志位追踪连接是否已被 SMTP 客户端接管
	// 这样可以确保在任何错误情况下都能正确清理资源
	connTakenOver := false
	defer func() {
		if !connTakenOver {
			conn.Close()
		}
	}()

	// 创建 SMTP 客户端
	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("创建 SMTP 客户端失败: %v", err))
	}
	connTakenOver = true // 连接已被 client 接管
	defer client.Close()

	// TLS 配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify, // 从配置读取
		ServerName:         c.config.SMTPHost,
	}

	// 升级到 TLS 连接
	if err := client.StartTLS(tlsConfig); err != nil {
		return errorx.Msg(fmt.Sprintf("STARTTLS 失败: %v", err))
	}

	// SMTP 认证
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return errorx.Msg(fmt.Sprintf("SMTP 认证失败: %v", err))
		}
	}

	// 设置发件人
	if err := client.Mail(c.config.Username); err != nil {
		return errorx.Msg(fmt.Sprintf("设置发件人失败: %v", err))
	}

	// 设置收件人
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return errorx.Msg(fmt.Sprintf("设置收件人 %s 失败: %v", recipient, err))
		}
	}

	// 写入邮件内容
	writer, err := client.Data()
	if err != nil {
		return errorx.Msg(fmt.Sprintf("获取数据写入器失败: %v", err))
	}

	if _, err := writer.Write(msg); err != nil {
		return errorx.Msg(fmt.Sprintf("写入邮件数据失败: %v", err))
	}

	if err := writer.Close(); err != nil {
		return errorx.Msg(fmt.Sprintf("关闭数据写入器失败: %v", err))
	}

	return client.Quit()
}

// TestConnection 测试邮件服务器连接
// toEmail 参数指定测试邮件的收件人
func (c *emailClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	startTime := time.Now()

	// 必须提供测试收件人
	if len(toEmail) == 0 {
		return &TestResult{
			Success:   false,
			Message:   "请提供测试收件人邮箱",
			Latency:   time.Since(startTime),
			Timestamp: time.Now(),
			Error:     errorx.Msg("收件人列表为空"),
		}, errorx.Msg("收件人列表为空")
	}

	// 创建测试消息
	testOpts := &NotificationOptions{
		PortalName: "测试平台",
		PortalUrl:  "https://example.com",
		Title:      "邮件连接测试",
		Content:    "这是一条测试消息，用于验证邮件告警配置是否正确。",
		Mentions: &MentionConfig{
			EmailTo: toEmail,
		},
	}

	result, err := c.SendNotification(ctx, testOpts)

	latency := time.Since(startTime)

	if err != nil {
		c.updateHealthStatus(false, err)
		return &TestResult{
			Success:   false,
			Message:   fmt.Sprintf("测试失败: %v", err),
			Latency:   latency,
			Timestamp: time.Now(),
			Error:     err,
			Details: map[string]interface{}{
				"smtpHost":           c.config.SMTPHost,
				"smtpPort":           c.config.SMTPPort,
				"username":           c.config.Username,
				"useTLS":             c.config.ShouldUseTLS(),
				"useStartTLS":        c.config.UseStartTLS,
				"insecureSkipVerify": c.config.InsecureSkipVerify,
				"recipients":         toEmail,
			},
		}, err
	}

	c.updateHealthStatus(true, nil)
	return &TestResult{
		Success:   result.Success,
		Message:   fmt.Sprintf("邮件连接测试成功，已发送至: %s", strings.Join(toEmail, ", ")),
		Latency:   latency,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"smtpHost":           c.config.SMTPHost,
			"smtpPort":           c.config.SMTPPort,
			"username":           c.config.Username,
			"useTLS":             c.config.ShouldUseTLS(),
			"useStartTLS":        c.config.UseStartTLS,
			"insecureSkipVerify": c.config.InsecureSkipVerify,
			"recipients":         toEmail,
		},
	}, nil
}

// HealthCheck 健康检查
// 检查配置是否完整
func (c *emailClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, errorx.Msg("客户端已关闭")
	}

	// 检查 SMTP 配置
	if c.config.SMTPHost == "" || c.config.SMTPPort == 0 {
		c.updateHealthStatus(false, errorx.Msg("SMTP 配置不完整"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "SMTP 配置不完整",
			LastCheckTime: time.Now(),
		}, errorx.Msg("SMTP 配置不完整")
	}

	// 检查认证信息
	if c.config.Username == "" {
		c.updateHealthStatus(false, errorx.Msg("认证信息不完整"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "认证信息不完整",
			LastCheckTime: time.Now(),
		}, errorx.Msg("认证信息不完整")
	}

	c.updateHealthStatus(true, nil)
	// 记录 Prometheus 指标
	GlobalMetricsCollector.RecordChannelHealth("email", c.baseClient.GetUUID(), c.baseClient.GetName(), true)
	return &HealthStatus{
		Healthy:       true,
		Message:       "邮件客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// validateEmailConfig 验证邮件配置
func validateEmailConfig(config *EmailConfig) error {
	if config.SMTPHost == "" {
		return errorx.Msg("SMTP 服务器地址不能为空")
	}

	if config.SMTPPort == 0 {
		return errorx.Msg("SMTP 端口不能为空")
	}

	if config.Username == "" {
		return errorx.Msg("用户名不能为空")
	}

	if config.Password == "" {
		return errorx.Msg("密码不能为空")
	}

	// 验证端口范围
	if config.SMTPPort < 1 || config.SMTPPort > 65535 {
		return errorx.Msg("SMTP 端口必须在 1-65535 范围内")
	}

	// 验证邮箱格式
	if !isValidEmail(config.Username) {
		return errorx.Msg("用户名必须是有效的邮箱地址")
	}

	// 验证 From 字段格式（如果提供）
	if config.From != "" && !isValidEmailOrDisplayName(config.From) {
		return errorx.Msg("From 字段格式不正确，应为邮箱地址或 '显示名 <邮箱>' 格式")
	}

	return nil
}

// optimizeEmailConfig 根据常见邮箱提供商优化配置
func optimizeEmailConfig(config *EmailConfig) {
	// 根据 SMTP 主机自动设置最佳配置
	switch strings.ToLower(config.SMTPHost) {
	case "smtp.163.com":
		optimizeFor163(config)
	case "smtp.qq.com":
		optimizeForQQ(config)
	case "smtp.gmail.com":
		optimizeForGmail(config)
	case "smtp.outlook.com", "smtp-mail.outlook.com":
		optimizeForOutlook(config)
	case "smtp.sina.com":
		optimizeForSina(config)
	case "smtp.126.com":
		optimizeFor126(config)
	default:
		// 通用优化
		optimizeGeneric(config)
	}
}

// optimizeFor163 优化 163 邮箱配置
func optimizeFor163(config *EmailConfig) {
	// 163 邮箱推荐配置
	if config.SMTPPort == 0 {
		config.SMTPPort = 465 // 默认使用 SSL 端口
		config.UseTLS = true
		config.UseStartTLS = false
	} else if config.SMTPPort == 25 {
		// 25 端口通常不加密
		config.UseTLS = false
		config.UseStartTLS = false
	} else if config.SMTPPort == 587 {
		// 587 端口使用 STARTTLS
		config.UseTLS = false
		config.UseStartTLS = true
	} else if config.SMTPPort == 465 {
		// 465 端口使用 SSL/TLS
		config.UseTLS = true
		config.UseStartTLS = false
	}

	// 163 邮箱通常需要授权码而不是密码
	if config.From == "" {
		config.From = config.Username
	}
}

// optimizeForQQ 优化 QQ 邮箱配置
func optimizeForQQ(config *EmailConfig) {
	if config.SMTPPort == 0 {
		config.SMTPPort = 587 // QQ 邮箱推荐 587 端口
		config.UseStartTLS = true
		config.UseTLS = false
	} else if config.SMTPPort == 465 {
		config.UseTLS = true
		config.UseStartTLS = false
	} else if config.SMTPPort == 587 {
		config.UseStartTLS = true
		config.UseTLS = false
	}

	if config.From == "" {
		config.From = config.Username
	}
}

// optimizeForGmail 优化 Gmail 配置
func optimizeForGmail(config *EmailConfig) {
	if config.SMTPPort == 0 {
		config.SMTPPort = 587 // Gmail 推荐 587 端口
		config.UseStartTLS = true
		config.UseTLS = false
	} else if config.SMTPPort == 465 {
		config.UseTLS = true
		config.UseStartTLS = false
	} else if config.SMTPPort == 587 {
		config.UseStartTLS = true
		config.UseTLS = false
	}

	if config.From == "" {
		config.From = config.Username
	}
}

// optimizeForOutlook 优化 Outlook 配置
func optimizeForOutlook(config *EmailConfig) {
	if config.SMTPPort == 0 {
		config.SMTPPort = 587 // Outlook 推荐 587 端口
		config.UseStartTLS = true
		config.UseTLS = false
	}

	if config.From == "" {
		config.From = config.Username
	}
}

// optimizeForSina 优化新浪邮箱配置
func optimizeForSina(config *EmailConfig) {
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
		config.UseStartTLS = true
		config.UseTLS = false
	}

	if config.From == "" {
		config.From = config.Username
	}
}

// optimizeFor126 优化 126 邮箱配置
func optimizeFor126(config *EmailConfig) {
	if config.SMTPPort == 0 {
		config.SMTPPort = 465
		config.UseTLS = true
		config.UseStartTLS = false
	}

	if config.From == "" {
		config.From = config.Username
	}
}

// optimizeGeneric 通用优化
func optimizeGeneric(config *EmailConfig) {
	// 根据端口自动设置加密方式
	if config.SMTPPort == 465 && !config.UseTLS && !config.UseSSL {
		config.UseTLS = true
		config.UseStartTLS = false
	} else if config.SMTPPort == 587 && !config.UseStartTLS {
		config.UseStartTLS = true
		config.UseTLS = false
	}

	if config.From == "" {
		config.From = config.Username
	}
}

// isValidEmail 验证邮箱地址格式
func isValidEmail(email string) bool {
	// 简单的邮箱格式验证
	if email == "" {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	localPart := parts[0]
	domainPart := parts[1]

	if localPart == "" || domainPart == "" {
		return false
	}

	// 检查域名部分是否包含点
	if !strings.Contains(domainPart, ".") {
		return false
	}

	return true
}

// isValidEmailOrDisplayName 验证邮箱地址或显示名格式
func isValidEmailOrDisplayName(input string) bool {
	if input == "" {
		return false
	}

	// 如果是纯邮箱地址
	if isValidEmail(input) {
		return true
	}

	// 如果是 "显示名 <邮箱>" 格式
	if strings.Contains(input, "<") && strings.Contains(input, ">") {
		start := strings.Index(input, "<")
		end := strings.Index(input, ">")
		if start < end && start >= 0 && end > start+1 {
			email := input[start+1 : end]
			return isValidEmail(email)
		}
	}

	return false
}

// GetEmailConfigTemplate 获取常见邮箱提供商的配置模板
func GetEmailConfigTemplate(provider string) *EmailConfig {
	switch strings.ToLower(provider) {
	case "163", "163.com":
		return &EmailConfig{
			SMTPHost:           "smtp.163.com",
			SMTPPort:           465,
			UseTLS:             true,
			UseStartTLS:        false,
			InsecureSkipVerify: false,
		}
	case "qq", "qq.com":
		return &EmailConfig{
			SMTPHost:           "smtp.qq.com",
			SMTPPort:           587,
			UseTLS:             false,
			UseStartTLS:        true,
			InsecureSkipVerify: false,
		}
	case "gmail", "gmail.com":
		return &EmailConfig{
			SMTPHost:           "smtp.gmail.com",
			SMTPPort:           587,
			UseTLS:             false,
			UseStartTLS:        true,
			InsecureSkipVerify: false,
		}
	case "outlook", "outlook.com", "hotmail", "hotmail.com":
		return &EmailConfig{
			SMTPHost:           "smtp-mail.outlook.com",
			SMTPPort:           587,
			UseTLS:             false,
			UseStartTLS:        true,
			InsecureSkipVerify: false,
		}
	case "sina", "sina.com":
		return &EmailConfig{
			SMTPHost:           "smtp.sina.com",
			SMTPPort:           587,
			UseTLS:             false,
			UseStartTLS:        true,
			InsecureSkipVerify: false,
		}
	case "126", "126.com":
		return &EmailConfig{
			SMTPHost:           "smtp.126.com",
			SMTPPort:           465,
			UseTLS:             true,
			UseStartTLS:        false,
			InsecureSkipVerify: false,
		}
	default:
		return nil
	}
}

// GetEmailProviderHelp 获取邮箱提供商的配置帮助信息
func GetEmailProviderHelp(provider string) string {
	switch strings.ToLower(provider) {
	case "163", "163.com":
		return "163邮箱配置说明：\n" +
			"1. SMTP服务器: smtp.163.com\n" +
			"2. 推荐端口: 465 (SSL) 或 25 (无加密)\n" +
			"3. 用户名: 完整邮箱地址 (如: user@163.com)\n" +
			"4. 密码: 使用授权码，不是登录密码\n" +
			"5. 获取授权码: 登录163邮箱 → 设置 → POP3/SMTP/IMAP → 开启SMTP服务 → 获取授权码\n" +
			"6. 加密方式: 465端口使用SSL，25端口不加密"

	case "qq", "qq.com":
		return "QQ邮箱配置说明：\n" +
			"1. SMTP服务器: smtp.qq.com\n" +
			"2. 推荐端口: 587 (STARTTLS) 或 465 (SSL)\n" +
			"3. 用户名: 完整QQ邮箱地址 (如: 123456@qq.com)\n" +
			"4. 密码: 使用授权码，不是QQ密码\n" +
			"5. 获取授权码: 登录QQ邮箱 → 设置 → 账户 → 开启SMTP服务 → 生成授权码\n" +
			"6. 加密方式: 587端口使用STARTTLS，465端口使用SSL"

	case "gmail", "gmail.com":
		return "Gmail配置说明：\n" +
			"1. SMTP服务器: smtp.gmail.com\n" +
			"2. 推荐端口: 587 (STARTTLS) 或 465 (SSL)\n" +
			"3. 用户名: 完整Gmail地址 (如: user@gmail.com)\n" +
			"4. 密码: 使用应用专用密码，不是Google账户密码\n" +
			"5. 设置步骤: 启用两步验证 → 生成应用专用密码\n" +
			"6. 加密方式: 587端口使用STARTTLS，465端口使用SSL"

	case "outlook", "outlook.com", "hotmail", "hotmail.com":
		return "Outlook邮箱配置说明：\n" +
			"1. SMTP服务器: smtp-mail.outlook.com\n" +
			"2. 推荐端口: 587 (STARTTLS)\n" +
			"3. 用户名: 完整Outlook邮箱地址\n" +
			"4. 密码: Microsoft账户密码\n" +
			"5. 加密方式: 587端口使用STARTTLS"

	case "sina", "sina.com":
		return "新浪邮箱配置说明：\n" +
			"1. SMTP服务器: smtp.sina.com\n" +
			"2. 推荐端口: 587 (STARTTLS) 或 25 (无加密)\n" +
			"3. 用户名: 完整新浪邮箱地址\n" +
			"4. 密码: 邮箱密码或授权码\n" +
			"5. 加密方式: 587端口使用STARTTLS"

	case "126", "126.com":
		return "126邮箱配置说明：\n" +
			"1. SMTP服务器: smtp.126.com\n" +
			"2. 推荐端口: 465 (SSL) 或 25 (无加密)\n" +
			"3. 用户名: 完整126邮箱地址\n" +
			"4. 密码: 使用授权码，不是登录密码\n" +
			"5. 获取授权码: 登录126邮箱 → 设置 → POP3/SMTP/IMAP → 开启SMTP服务\n" +
			"6. 加密方式: 465端口使用SSL"

	default:
		return "通用SMTP配置说明：\n" +
			"1. 常用端口: 25(无加密), 587(STARTTLS), 465(SSL)\n" +
			"2. 加密方式: 根据端口选择对应的加密方式\n" +
			"3. 认证: 大多数邮箱需要用户名和密码认证\n" +
			"4. 用户名: 通常是完整的邮箱地址\n" +
			"5. 密码: 可能需要使用授权码而不是登录密码"
	}
}
