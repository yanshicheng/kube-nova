package watch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
)

// IncrementalSyncService 增量同步服务
type IncrementalSyncService struct {
	svcCtx *svc.ServiceContext // 服务上下文，包含所有依赖

	rpcAddr        string        // RPC 服务地址，用于健康检查
	startupTimeout time.Duration // 启动超时时间
	checkInterval  time.Duration // 健康检查间隔

	ctx    context.Context    // 服务上下文
	cancel context.CancelFunc // 用于取消服务
}

// IncrementalSyncServiceConfig 服务配置
type IncrementalSyncServiceConfig struct {
	SvcCtx         *svc.ServiceContext
	RPCAddr        string
	StartupTimeout time.Duration // 默认 60s
	CheckInterval  time.Duration // 默认 500ms
}

// NewIncrementalSyncService 创建增量同步服务实例
func NewIncrementalSyncService(cfg IncrementalSyncServiceConfig) *IncrementalSyncService {
	// 设置默认值
	if cfg.StartupTimeout <= 0 {
		cfg.StartupTimeout = 60 * time.Second
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 500 * time.Millisecond
	}

	// 处理地址格式：将 0.0.0.0 替换为 127.0.0.1
	rpcAddr := cfg.RPCAddr
	if strings.HasPrefix(rpcAddr, "0.0.0.0:") {
		rpcAddr = strings.Replace(rpcAddr, "0.0.0.0:", "127.0.0.1:", 1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &IncrementalSyncService{
		svcCtx:         cfg.SvcCtx,
		rpcAddr:        rpcAddr,
		startupTimeout: cfg.StartupTimeout,
		checkInterval:  cfg.CheckInterval,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动增量同步服务
func (s *IncrementalSyncService) Start() {
	logx.Info("[IncrementalSyncService] 服务启动中，等待 RPC 服务就绪...")

	// 等待 RPC 服务就绪（通过实际调用 RPC 方法验证）
	if err := s.waitForRPCReady(); err != nil {
		logx.Errorf("[IncrementalSyncService] 等待 RPC 服务就绪失败: %v", err)
		return
	}

	logx.Info("[IncrementalSyncService] RPC 服务已就绪，开始初始化增量同步...")

	// 设置事件处理器（Handler）
	if !SetupIncrementalSync(s.svcCtx) {
		logx.Error("[IncrementalSyncService] 增量同步设置失败")
		return
	}

	// 启动增量同步管理器
	if err := StartIncrementalSync(s.svcCtx); err != nil {
		logx.Errorf("[IncrementalSyncService] 增量同步启动失败: %v", err)
		return
	}

	logx.Info("[IncrementalSyncService] 增量同步启动成功")

	//  阻塞等待，直到收到停止信号
	<-s.ctx.Done()

	logx.Info("[IncrementalSyncService] 收到停止信号，服务即将关闭")
}

// Stop 停止增量同步服务
func (s *IncrementalSyncService) Stop() {
	logx.Info("[IncrementalSyncService] 正在停止增量同步服务...")

	// 取消 context，让 Start() 方法中的阻塞结束
	s.cancel()

	// 停止增量同步管理器
	if err := StopIncrementalSync(s.svcCtx); err != nil {
		logx.Errorf("[IncrementalSyncService] 停止增量同步失败: %v", err)
	} else {
		logx.Info("[IncrementalSyncService] 增量同步服务已停止")
	}
}

// waitForRPCReady 等待 RPC 服务就绪
func (s *IncrementalSyncService) waitForRPCReady() error {
	deadline := time.Now().Add(s.startupTimeout)

	logx.Infof("[IncrementalSyncService] 开始检测 RPC 服务 (%s)，超时时间: %v",
		s.rpcAddr, s.startupTimeout)

	attempt := 0
	var lastErr error

	for time.Now().Before(deadline) {
		attempt++

		// 检查是否已经收到停止信号
		select {
		case <-s.ctx.Done():
			return fmt.Errorf("服务被取消")
		default:
		}

		// 调用 rpc 方法 判断 服务是否就绪
		if err := s.checkRPCHealth(); err != nil {
			lastErr = err
			// 连接失败，等待后重试
			if attempt%10 == 0 {
				logx.Infof("[IncrementalSyncService] 等待 RPC 服务就绪，已尝试 %d 次, 最后错误: %v",
					attempt, err)
			}
			time.Sleep(s.checkInterval)
			continue
		}

		// RPC 调用成功，服务已就绪
		logx.Infof("[IncrementalSyncService] RPC 服务就绪，尝试次数: %d", attempt)
		return nil
	}

	return fmt.Errorf("等待 RPC 服务就绪超时 (地址: %s, 超时: %v, 最后错误: %v)",
		s.rpcAddr, s.startupTimeout, lastErr)
}

// checkRPCHealth 通过实际调用 RPC 方法来检查服务是否就绪
func (s *IncrementalSyncService) checkRPCHealth() error {
	rpcConf := zrpc.RpcClientConf{
		Endpoints: []string{s.rpcAddr},
		NonBlock:  true,
		Timeout:   3000, // 3 秒超时
	}

	// 尝试创建客户端
	client, err := zrpc.NewClient(rpcConf, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))
	if err != nil {
		return fmt.Errorf("创建 RPC 客户端失败: %w", err)
	}
	defer client.Conn().Close()

	// 创建 ManagerService 客户端
	managerClient := managerservice.NewManagerService(client)

	// 调用一个轻量级的 RPC 方法来验证服务是否可用
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = managerClient.GetClusterAuthInfo(ctx, &managerservice.GetClusterAuthInfoReq{
		ClusterUuid: "__health_check__",
	})

	if err != nil {
		errStr := err.Error()
		// 如果错误是 "服务不可用" 或 "连接被拒绝"，说明服务还没准备好
		if strings.Contains(errStr, "服务不可用") ||
			strings.Contains(errStr, "unavailable") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "transport") {
			return fmt.Errorf("RPC 服务未就绪: %w", err)
		}
		// 其他错误（如"集群不存在"）说明服务已经在正常处理请求了
		logx.Debugf("[IncrementalSyncService] RPC 健康检查返回业务错误（这是正常的）: %v", err)
	}

	return nil
}

// GetStats 获取服务状态
func (s *IncrementalSyncService) GetStats() *incremental.ManagerStats {
	if s.svcCtx == nil || s.svcCtx.IncrementalSyncManager == nil {
		return nil
	}
	return s.svcCtx.IncrementalSyncManager.GetStats()
}

// IsRunning 检查服务是否在运行
func (s *IncrementalSyncService) IsRunning() bool {
	if s.svcCtx == nil || s.svcCtx.IncrementalSyncManager == nil {
		return false
	}
	return s.svcCtx.IncrementalSyncManager.IsRunning()
}
