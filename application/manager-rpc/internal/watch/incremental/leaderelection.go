package incremental

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const (
	// Leader Election 默认配置
	DefaultLeaseDuration = 15 * time.Second
	DefaultRenewDeadline = 10 * time.Second
	DefaultRetryPeriod   = 2 * time.Second

	// Lease 资源名称和命名空间
	DefaultLeaseName      = "manager-rpc-watch-leader"
	DefaultLeaseNamespace = "kube-nova"
)

// LeaderElectionConfig Leader Election 配置
type LeaderElectionConfig struct {
	// Lease 资源配置
	LeaseName      string // Lease 名称，默认 "manager-rpc-watch-leader"
	LeaseNamespace string // Lease 命名空间，默认 "kube-nova"

	// Leader Election 时间配置
	LeaseDuration time.Duration // 租约有效期，默认 15s
	RenewDeadline time.Duration // 续约超时，默认 10s
	RetryPeriod   time.Duration // 重试间隔，默认 2s

	// K8s 客户端（用于创建 Lease）
	KubeClient kubernetes.Interface

	// 回调函数
	OnStartedLeading func(ctx context.Context) // 成为 Leader 时调用
	OnStoppedLeading func()                    // 失去 Leader 时调用
	OnNewLeader      func(identity string)     // 新 Leader 产生时调用（可选）
}

// LeaderElector Leader 选举器
type LeaderElector struct {
	config   LeaderElectionConfig
	identity string // 当前节点身份标识
	isLeader int32  // 是否是 Leader（使用 atomic）

	ctx    context.Context
	cancel context.CancelFunc
}

// NewLeaderElector 创建 Leader 选举器
func NewLeaderElector(cfg LeaderElectionConfig) (*LeaderElector, error) {
	// 设置默认值
	if cfg.LeaseName == "" {
		cfg.LeaseName = DefaultLeaseName
	}
	if cfg.LeaseNamespace == "" {
		cfg.LeaseNamespace = DefaultLeaseNamespace
	}
	if cfg.LeaseDuration == 0 {
		cfg.LeaseDuration = DefaultLeaseDuration
	}
	if cfg.RenewDeadline == 0 {
		cfg.RenewDeadline = DefaultRenewDeadline
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = DefaultRetryPeriod
	}

	// 验证配置
	if cfg.KubeClient == nil {
		return nil, fmt.Errorf("KubeClient is required")
	}
	if cfg.OnStartedLeading == nil {
		return nil, fmt.Errorf("OnStartedLeading callback is required")
	}
	if cfg.OnStoppedLeading == nil {
		return nil, fmt.Errorf("OnStoppedLeading callback is required")
	}

	// 验证时间配置的合理性
	if cfg.RenewDeadline >= cfg.LeaseDuration {
		return nil, fmt.Errorf("RenewDeadline (%v) must be less than LeaseDuration (%v)",
			cfg.RenewDeadline, cfg.LeaseDuration)
	}
	if cfg.RetryPeriod >= cfg.RenewDeadline {
		return nil, fmt.Errorf("RetryPeriod (%v) must be less than RenewDeadline (%v)",
			cfg.RetryPeriod, cfg.RenewDeadline)
	}

	// 生成节点身份标识
	identity, err := generateIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity: %w", err)
	}

	return &LeaderElector{
		config:   cfg,
		identity: identity,
		isLeader: 0,
	}, nil
}

// Start 启动 Leader 选举
// 此方法会阻塞，直到 context 被取消
func (le *LeaderElector) Start(ctx context.Context) error {
	le.ctx, le.cancel = context.WithCancel(ctx)

	logx.Infof("[LeaderElection] 启动 Leader 选举, identity=%s, lease=%s/%s",
		le.identity, le.config.LeaseNamespace, le.config.LeaseName)

	// 创建 Lease 锁
	lock, err := le.createLock()
	if err != nil {
		return fmt.Errorf("failed to create lease lock: %w", err)
	}

	// 配置 Leader Election
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   le.config.LeaseDuration,
		RenewDeadline:   le.config.RenewDeadline,
		RetryPeriod:     le.config.RetryPeriod,
		ReleaseOnCancel: true, // context 取消时释放 Lease
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				atomic.StoreInt32(&le.isLeader, 1)
				logx.Infof("[LeaderElection] 成为 Leader, identity=%s", le.identity)
				le.config.OnStartedLeading(ctx)
			},
			OnStoppedLeading: func() {
				atomic.StoreInt32(&le.isLeader, 0)
				logx.Infof("[LeaderElection] 失去 Leader 身份, identity=%s", le.identity)
				le.config.OnStoppedLeading()
			},
			OnNewLeader: func(identity string) {
				if identity == le.identity {
					logx.Infof("[LeaderElection] 当前节点成为 Leader")
				} else {
					logx.Infof("[LeaderElection] 新 Leader 产生: %s", identity)
				}
				if le.config.OnNewLeader != nil {
					le.config.OnNewLeader(identity)
				}
			},
		},
	}

	// 创建 LeaderElector（不使用 RunOrDie，避免 panic）
	elector, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	// 运行 Leader Election（阻塞，直到 context 取消）
	// Run 方法会持续参与选举，失去 Leader 后会重新竞争
	elector.Run(le.ctx)

	logx.Infof("[LeaderElection] Leader 选举已停止, identity=%s", le.identity)
	return nil
}

// Stop 停止 Leader 选举
func (le *LeaderElector) Stop() {
	logx.Infof("[LeaderElection] 停止 Leader 选举, identity=%s", le.identity)
	if le.cancel != nil {
		le.cancel()
	}
}

// IsLeader 返回当前节点是否是 Leader
func (le *LeaderElector) IsLeader() bool {
	return atomic.LoadInt32(&le.isLeader) == 1
}

// GetIdentity 返回当前节点身份标识
func (le *LeaderElector) GetIdentity() string {
	return le.identity
}

// createLock 创建 Lease 锁
func (le *LeaderElector) createLock() (resourcelock.Interface, error) {
	return resourcelock.New(
		resourcelock.LeasesResourceLock,
		le.config.LeaseNamespace,
		le.config.LeaseName,
		le.config.KubeClient.CoreV1(),
		le.config.KubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: le.identity,
		},
	)
}

// generateIdentity 生成节点身份标识
// 格式: hostname-pid-timestamp
func generateIdentity() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// 使用 hostname + pid 作为身份标识
	// 这样即使同一台机器上运行多个实例也能区分
	return fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano()%10000), nil
}

// LeaderElectionInfo Leader 选举信息（用于状态查询）
type LeaderElectionInfo struct {
	Identity       string `json:"identity"`
	IsLeader       bool   `json:"isLeader"`
	LeaseName      string `json:"leaseName"`
	LeaseNamespace string `json:"leaseNamespace"`
}

// GetInfo 获取 Leader 选举信息
func (le *LeaderElector) GetInfo() *LeaderElectionInfo {
	return &LeaderElectionInfo{
		Identity:       le.identity,
		IsLeader:       atomic.LoadInt32(&le.isLeader) == 1,
		LeaseName:      le.config.LeaseName,
		LeaseNamespace: le.config.LeaseNamespace,
	}
}

// GetCurrentLeader 获取当前 Leader 信息
func (le *LeaderElector) GetCurrentLeader(ctx context.Context) (string, error) {
	lease, err := le.config.KubeClient.CoordinationV1().Leases(le.config.LeaseNamespace).Get(
		ctx, le.config.LeaseName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get lease: %w", err)
	}

	if lease.Spec.HolderIdentity == nil {
		return "", nil
	}

	return *lease.Spec.HolderIdentity, nil
}
