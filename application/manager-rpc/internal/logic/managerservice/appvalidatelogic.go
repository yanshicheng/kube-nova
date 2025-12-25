package managerservicelogic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppValidateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAppValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppValidateLogic {
	return &AppValidateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// buildFullURL 根据 protocol、appUrl、port 构建完整的 URL
func (l *AppValidateLogic) buildFullURL(app *model.OnecClusterApp) string {
	// 移除 appUrl 中可能存在的协议前缀
	host := strings.TrimPrefix(app.AppUrl, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSuffix(host, "/")

	// 构建完整 URL
	protocol := strings.ToLower(app.Protocol)
	if protocol == "" {
		protocol = "http"
	}

	// 判断是否需要添加端口
	// 如果是默认端口（http:80, https:443）可以省略
	if app.Port > 0 {
		if (protocol == "http" && app.Port != 80) || (protocol == "https" && app.Port != 443) {
			return fmt.Sprintf("%s://%s:%d", protocol, host, app.Port)
		}
	}

	return fmt.Sprintf("%s://%s", protocol, host)
}

// AppValidate 验证应用连接和状态是否正常
func (l *AppValidateLogic) AppValidate(in *pb.ClusterAppValidateReq) (*pb.ClusterAppValidateResp, error) {

	if in.Id == 0 {
		l.Errorf("应用ID不能为空")
		return nil, errorx.Msg("应用ID不能为空")
	}

	app, err := l.svcCtx.OnecClusterAppModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用不存在 [appId=%d]", in.Id)
			return nil, errorx.Msg("指定的应用不存在")
		}
		l.Errorf("查询应用失败: %v", err)
		return nil, errorx.Msg("查询应用信息失败")
	}

	l.Infof("应用查询成功 [appName=%s, appUrl=%s, port=%d, protocol=%s]",
		app.AppName, app.AppUrl, app.Port, app.Protocol)

	// 3. 验证集群是否存在
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, app.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用关联的集群不存在 [clusterUuid=%s]", app.ClusterUuid)
			return nil, errorx.Msg("应用关联的集群不存在")
		}
		l.Errorf("查询关联集群失败: %v", err)
		return nil, errorx.Msg("查询关联集群失败")
	}
	l.Infof("关联集群验证成功 [clusterName=%s]", cluster.Name)

	// 4. 执行应用连接验证
	var validationErr error

	// 记录验证开始时间
	validateStartTime := time.Now()

	switch strings.ToLower(app.Protocol) {
	case "http", "https":
		validationErr = l.validateHTTPApp(app)
	case "grpc":
		validationErr = l.validateGRPCApp(app)
	default:
		l.Errorf("不支持的协议类型 [protocol=%s]", app.Protocol)
		validationErr = fmt.Errorf("不支持的协议类型: %s", app.Protocol)
	}

	validateDuration := time.Since(validateStartTime)
	l.Infof("验证耗时: %v", validateDuration)

	var newStatus int64
	var statusDesc string

	if validationErr != nil {
		newStatus = 0 // 异常状态
		statusDesc = "异常"
		l.Errorf("应用验证失败 [appId=%d, error=%v]", in.Id, validationErr)
	} else {
		newStatus = 1 // 正常状态
		statusDesc = "正常"
		l.Infof("应用验证成功 [appId=%d]", in.Id)
	}

	if app.Status != newStatus {
		app.Status = newStatus

		err = l.svcCtx.OnecClusterAppModel.Update(l.ctx, app)
		if err != nil {
			l.Errorf("更新应用状态失败: %v", err)
			return nil, errorx.Msg("更新应用状态失败")
		}
		l.Infof("应用状态更新成功 [appId=%d, newStatus=%s]", in.Id, statusDesc)
	} else {
		l.Infof("应用状态无变化，跳过数据库更新 [currentStatus=%s]", statusDesc)
	}

	if validationErr != nil {
		l.Errorf("应用验证最终失败 [appId=%d]", in.Id)
		return nil, errorx.Msg(fmt.Sprintf("应用验证失败: %v", validationErr))
	}

	l.Infof("应用验证操作完成 [appId=%d, 结果=%s]", in.Id, statusDesc)
	return &pb.ClusterAppValidateResp{}, nil
}

// validateHTTPApp 验证HTTP/HTTPS应用连接
func (l *AppValidateLogic) validateHTTPApp(app *model.OnecClusterApp) error {
	l.Infof("开始验证HTTP应用 [appUrl=%s, port=%d, protocol=%s]", app.AppUrl, app.Port, app.Protocol)

	// 检查URL是否为空
	if app.AppUrl == "" {
		return fmt.Errorf("应用地址为空")
	}

	// 根据 protocol、appUrl、port 构建完整 URL
	fullURL := l.buildFullURL(app)

	// 创建 HTTP Transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	// 如果配置了跳过TLS验证
	if app.InsecureSkipVerify == 1 {
		l.Infof("配置跳过TLS证书验证")
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	// 发送HTTP请求进行连接测试
	l.Infof("发送HTTP请求测试连接 [url=%s]", fullURL)

	// 构建请求
	req, err := http.NewRequestWithContext(l.ctx, "GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 添加认证信息（如果需要）
	if app.AuthEnabled == 1 {
		switch app.AuthType {
		case "basic":
			if app.Username != "" && app.Password != "" {
				req.SetBasicAuth(app.Username, app.Password)
				l.Infof("添加Basic认证")
			}
		case "bearer":
			if app.Token.Valid && app.Token.String != "" {
				req.Header.Set("Authorization", "Bearer "+app.Token.String)
				l.Infof("添加Bearer认证")
			}
		}
	}

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 500 {
		return fmt.Errorf("服务器错误: HTTP %d", resp.StatusCode)
	}

	l.Infof("HTTP验证成功 [statusCode=%d]", resp.StatusCode)
	return nil
}

// validateGRPCApp 验证GRPC应用连接
func (l *AppValidateLogic) validateGRPCApp(app *model.OnecClusterApp) error {
	l.Infof("开始验证GRPC应用 [appUrl=%s, port=%d]", app.AppUrl, app.Port)

	// 检查基本配置
	if app.AppUrl == "" {
		return fmt.Errorf("GRPC服务地址为空")
	}

	if app.Port <= 0 {
		return fmt.Errorf("GRPC服务端口无效")
	}

	// 移除可能存在的协议前缀
	host := strings.TrimPrefix(app.AppUrl, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "grpc://")
	host = strings.TrimSuffix(host, "/")

	// 构建GRPC服务地址
	grpcAddr := fmt.Sprintf("%s:%d", host, app.Port)
	l.Infof("GRPC服务地址: %s", grpcAddr)

	// 尝试 TCP 连接测试
	conn, err := net.DialTimeout("tcp", grpcAddr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("GRPC连接失败: %v", err)
	}
	defer conn.Close()

	l.Infof("GRPC验证成功 [addr=%s]", grpcAddr)
	return nil
}
