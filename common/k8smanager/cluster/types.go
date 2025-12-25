package cluster

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// 认证相关
// AuthType 认证类型
type AuthType string

const (
	// AuthTypeKubeConfig 使用 kubeconfig 文件进行认证
	AuthTypeKubeConfig AuthType = "kubeconfig"
	// AUthTypeInCluster 使用 incluster 认证
	AuthTypeInCluster AuthType = "incluster"
	// AuthTypeToken 使用 token 进行认证
	AuthTypeToken AuthType = "token"
	// AuthTypeCertificate 使用证书进行认证
	AuthTypeCertificate AuthType = "certificate"
)

type ClusterVersionInfo struct {
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	GitCommit       string `json:"gitCommit"`
	VersionBuildAt  int64  `json:"buildAt"`
	ClusterCreateAt int64  `json:"createAt"`
}

// Client 集群接口
type Client interface {
	// 基本信息
	GetID() string     // 获取集群 ID
	GetName() string   // 获取集群名称
	GetConfig() Config // 获取集群配置
	//Remove(string) error
	// 获取版本信息
	GetVersionInfo() (*ClusterVersionInfo, error)
	// 设置日志上下文
	WithContext(ctx context.Context) Client // 设置日志上下文

	// 原始客户端接口
	GetKubeClient() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
	GetClientSet() *kubernetes.Clientset
	GetRESTConfig() *rest.Config
	GetInformerFactory() informers.SharedInformerFactory
	GetNetworkInfo() (*ClusterNetworkInfo, error)
	// 获取日志
	GetLogger() logx.Logger

	// 资源操作器
	operatorInterface

	// 关闭生命周期
	Close() error

	// 健康状态
	IsHealthy() bool
	HealthCheck(ctx context.Context) error

	// 缓存同步
	WaitForCacheSync(ctx context.Context) error
	HasSynced() bool
}

// Manager 集群管理器接口
type Manager interface {
	// 集群管理
	AddCluster(config Config) error                              // 添加集群
	RemoveCluster(id string) error                               // 移除集群
	UpdateCluster(config Config) error                           // 更新集群
	GetCluster(ctx context.Context, uuid string) (Client, error) // 获取集群

	//集群列表
	ListClusters() []Client
	// 获取集群数量
	GetClusterCount() int

	// 生命周期
	Close() error
}
