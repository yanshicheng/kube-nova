package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// K8sLeaderElectionConfig K8s Leader 选举配置
type K8sLeaderElectionConfig struct {
	// K8s 客户端
	KubeClient kubernetes.Interface

	// Lease 资源配置
	LeaseName      string
	LeaseNamespace string

	// 时间配置
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration

	// 节点标识
	Identity string

	// 回调函数
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func()
	OnNewLeader      func(identity string)
}

// LeaderElector Leader 选举器
type LeaderElector struct {
	config   K8sLeaderElectionConfig
	elector  *leaderelection.LeaderElector
	logger   logx.Logger
	stopChan chan struct{}
}

// NewLeaderElector 创建 Leader 选举器
func NewLeaderElector(cfg K8sLeaderElectionConfig) (*LeaderElector, error) {
	if cfg.KubeClient == nil {
		return nil, errorx.Msg("KubeClient is required")
	}
	if cfg.LeaseName == "" {
		return nil, errorx.Msg("LeaseName is required")
	}
	if cfg.LeaseNamespace == "" {
		return nil, errorx.Msg("LeaseNamespace is required")
	}
	if cfg.Identity == "" {
		return nil, errorx.Msg("Identity is required")
	}

	// 设置默认值
	if cfg.LeaseDuration == 0 {
		cfg.LeaseDuration = 15 * time.Second
	}
	if cfg.RenewDeadline == 0 {
		cfg.RenewDeadline = 10 * time.Second
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = 2 * time.Second
	}

	le := &LeaderElector{
		config:   cfg,
		logger:   logx.WithContext(context.Background()),
		stopChan: make(chan struct{}),
	}

	// 创建 Lease 资源锁
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      cfg.LeaseName,
			Namespace: cfg.LeaseNamespace,
		},
		Client: cfg.KubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: cfg.Identity,
		},
	}

	// 创建 Leader Election 配置
	leConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   cfg.LeaseDuration,
		RenewDeadline:   cfg.RenewDeadline,
		RetryPeriod:     cfg.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				le.logger.Infof("[LeaderElection] 成为 Leader | Identity=%s", cfg.Identity)
				if cfg.OnStartedLeading != nil {
					cfg.OnStartedLeading(ctx)
				}
			},
			OnStoppedLeading: func() {
				le.logger.Infof("[LeaderElection] 失去 Leader 身份 | Identity=%s", cfg.Identity)
				if cfg.OnStoppedLeading != nil {
					cfg.OnStoppedLeading()
				}
			},
			OnNewLeader: func(identity string) {
				if identity != cfg.Identity {
					le.logger.Infof("[LeaderElection] 新 Leader 选出 | Leader=%s", identity)
				}
				if cfg.OnNewLeader != nil {
					cfg.OnNewLeader(identity)
				}
			},
		},
	}

	// 创建 Leader Elector
	elector, err := leaderelection.NewLeaderElector(leConfig)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("创建 Leader Elector 失败: %v", err))
	}

	le.elector = elector
	return le, nil
}

// Run 运行 Leader 选举（阻塞）
func (le *LeaderElector) Run(ctx context.Context) {
	le.logger.Infof("[LeaderElection] 开始 Leader 选举 | Identity=%s, Lease=%s/%s",
		le.config.Identity, le.config.LeaseNamespace, le.config.LeaseName)

	le.elector.Run(ctx)
}

// Stop 停止 Leader 选举
func (le *LeaderElector) Stop() {
	close(le.stopChan)
}

// IsLeader 检查当前是否是 Leader
func (le *LeaderElector) IsLeader() bool {
	return le.elector.IsLeader()
}

// GetLeader 获取当前 Leader 的 Identity
func (le *LeaderElector) GetLeader() string {
	return le.elector.GetLeader()
}
