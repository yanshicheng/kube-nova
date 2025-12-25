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
)

// emailClient 邮件告警客户端
type emailClient struct {
	*baseClient
	config *EmailConfig
}

// NewEmailClient 创建邮件客户端
func NewEmailClient(config Config) (FullChannel, error) {
	if config.Email == nil {
		return nil, fmt.Errorf("邮件配置不能为空")
	}

	bc := newBaseClient(config)
	client := &emailClient{
		baseClient: bc,
		config:     config.Email,
	}

	client.l.Infof("创建邮件告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
func (c *emailClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警邮件
func (c *emailClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("客户端已关闭")
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

	// 获取收件人
	to, cc, bcc := c.getRecipients(opts.Mentions)
	if len(to) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "收件人列表为空",
			Timestamp: time.Now(),
			Error:     fmt.Errorf("收件人列表为空"),
		}, fmt.Errorf("收件人列表为空")
	}

	// 使用格式化器构建消息
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)
	subject, body := formatter.FormatHTMLForEmail(opts, alerts)

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
	} else {
		c.recordFailure()
	}

	return result, err
}

// SendNotification 发送通用通知邮件
func (c *emailClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("客户端已关闭")
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

	// 获取收件人
	to, cc, bcc := c.getRecipients(opts.Mentions)
	if len(to) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "收件人列表为空",
			Timestamp: time.Now(),
			Error:     fmt.Errorf("收件人列表为空"),
		}, fmt.Errorf("收件人列表为空")
	}

	// 使用格式化器构建消息
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
	} else {
		c.recordFailure()
	}

	return result, err
}

// getRecipients 获取收件人列表
func (c *emailClient) getRecipients(mentions *MentionConfig) (to, cc, bcc []string) {
	if mentions == nil {
		return nil, nil, nil
	}

	to = mentions.EmailTo
	cc = mentions.EmailCC
	bcc = mentions.EmailBCC

	return to, cc, bcc
}

// sendEmail 发送邮件
func (c *emailClient) sendEmail(subject, body string, to, cc, bcc []string) (*SendResult, error) {
	// 构建邮件头
	from := c.config.Username
	if c.config.From != "" {
		from = c.config.From
	}

	msg := c.buildEmailMessage(from, to, cc, subject, body)

	// 合并所有收件人
	allRecipients := make([]string, 0)
	allRecipients = append(allRecipients, to...)
	allRecipients = append(allRecipients, cc...)
	allRecipients = append(allRecipients, bcc...)

	// SMTP 服务器地址
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)

	// 认证信息
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.SMTPHost)

	var err error
	// 使用 ShouldUseTLS() 方法判断，兼容 useTls 和 useSSL
	if c.config.ShouldUseTLS() {
		err = c.sendWithTLS(addr, auth, allRecipients, msg)
	} else if c.config.UseStartTLS {
		err = c.sendWithStartTLS(addr, auth, allRecipients, msg)
	} else {
		err = smtp.SendMail(addr, auth, c.config.Username, allRecipients, msg)
	}

	if err != nil {
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	c.l.Infof("[邮件] 发送成功 | UUID: %s | 收件人: %v", c.baseClient.GetUUID(), to)
	return &SendResult{
		Success:   true,
		Message:   fmt.Sprintf("邮件发送成功，收件人数: %d", len(allRecipients)),
		Timestamp: time.Now(),
	}, nil
}

// buildEmailMessage 构建邮件消息
func (c *emailClient) buildEmailMessage(from string, to, cc []string, subject, body string) []byte {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ",")))
	if len(cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cc, ",")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return []byte(msg.String())
}

// sendWithTLS 使用 TLS 发送邮件（适用于 465 端口）
func (c *emailClient) sendWithTLS(addr string, auth smtp.Auth, recipients []string, msg []byte) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // 163邮箱等需要跳过证书验证
		ServerName:         c.config.SMTPHost,
	}

	// 使用带超时的连接
	dialer := &net.Dialer{
		Timeout: c.baseClient.config.Options.Timeout,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS 连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
	}
	defer client.Close()

	// 认证
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}

	// 设置发件人
	if err := client.Mail(c.config.Username); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	// 设置收件人
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人 %s 失败: %w", recipient, err)
		}
	}

	// 写入邮件内容
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}

	if _, err := writer.Write(msg); err != nil {
		return fmt.Errorf("写入邮件数据失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("关闭数据写入器失败: %w", err)
	}

	return client.Quit()
}

// sendWithStartTLS 使用 STARTTLS 发送邮件（适用于 587 端口）
func (c *emailClient) sendWithStartTLS(addr string, auth smtp.Auth, recipients []string, msg []byte) error {
	// 使用带超时的连接
	dialer := &net.Dialer{
		Timeout: c.baseClient.config.Options.Timeout,
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}

	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
	}
	defer client.Close()

	// STARTTLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         c.config.SMTPHost,
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS 失败: %w", err)
	}

	// 认证
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}

	// 设置发件人
	if err := client.Mail(c.config.Username); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	// 设置收件人
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人 %s 失败: %w", recipient, err)
		}
	}

	// 写入邮件内容
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}

	if _, err := writer.Write(msg); err != nil {
		return fmt.Errorf("写入邮件数据失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("关闭数据写入器失败: %w", err)
	}

	return client.Quit()
}

// TestConnection 测试连接
func (c *emailClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	startTime := time.Now()

	if len(toEmail) == 0 {
		return &TestResult{
			Success:   false,
			Message:   "请提供测试收件人邮箱",
			Latency:   time.Since(startTime),
			Timestamp: time.Now(),
			Error:     fmt.Errorf("收件人列表为空"),
		}, fmt.Errorf("收件人列表为空")
	}

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
				"smtpHost":    c.config.SMTPHost,
				"smtpPort":    c.config.SMTPPort,
				"username":    c.config.Username,
				"useTLS":      c.config.ShouldUseTLS(),
				"useStartTLS": c.config.UseStartTLS,
				"recipients":  toEmail,
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
			"smtpHost":    c.config.SMTPHost,
			"smtpPort":    c.config.SMTPPort,
			"username":    c.config.Username,
			"useTLS":      c.config.ShouldUseTLS(),
			"useStartTLS": c.config.UseStartTLS,
			"recipients":  toEmail,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *emailClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("客户端已关闭")
	}

	// 检查配置
	if c.config.SMTPHost == "" || c.config.SMTPPort == 0 {
		c.updateHealthStatus(false, fmt.Errorf("SMTP 配置不完整"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "SMTP 配置不完整",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("SMTP 配置不完整")
	}

	if c.config.Username == "" {
		c.updateHealthStatus(false, fmt.Errorf("认证信息不完整"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "认证信息不完整",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("认证信息不完整")
	}

	c.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "邮件客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}
