package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterTestConnectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterTestConnectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterTestConnectLogic {
	return &ClusterTestConnectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClusterTestConnect 测试集群连通性
func (l *ClusterTestConnectLogic) ClusterTestConnect(in *pb.ClusterTestConnectReq) (*pb.ClusterTestConnectResp, error) {
	// 生成临时集群 UUID
	clusterUuid := utils.NewUuid()

	l.Infof("开始测试集群连接，临时集群 ID: %s", clusterUuid)

	// 构建集群配置
	clusterConfig := cluster.Config{
		Name:     clusterUuid,
		ID:       clusterUuid,
		AuthType: cluster.AuthType(in.AuthType),
	}

	switch in.AuthType {
	case "kubeconfig":
		if in.KubeFile == "" {
			l.Error("kubeconfig 内容为空")
			return nil, errorx.Msg("kubeconfig 内容不能为空")
		}
		clusterConfig.KubeConfigData = in.KubeFile
		l.Info("使用 kubeconfig 认证模式")

	case "token":
		if in.ApiServerHost == "" {
			l.Error("API Server 地址为空")
			return nil, errorx.Msg("API Server 地址不能为空")
		}
		if in.Token == "" {
			l.Error("Token 为空")
			return nil, errorx.Msg("Token 不能为空")
		}
		clusterConfig.APIServer = in.ApiServerHost
		clusterConfig.Token = in.Token
		clusterConfig.Insecure = in.InsecureSkipVerify == 1
		l.Infof("使用 Token 认证模式，API Server: %s, Insecure: %v", in.ApiServerHost, clusterConfig.Insecure)

	case "certificate":
		if in.ApiServerHost == "" {
			l.Error("API Server 地址为空")
			return nil, errorx.Msg("API Server 地址不能为空")
		}
		if in.CertFile == "" || in.KeyFile == "" {
			l.Error("证书文件或密钥文件为空")
			return nil, errorx.Msg("证书文件和密钥文件不能为空")
		}
		clusterConfig.APIServer = in.ApiServerHost
		clusterConfig.CertFile = in.CertFile
		clusterConfig.KeyFile = in.KeyFile
		clusterConfig.CAFile = in.CaFile
		clusterConfig.Insecure = in.InsecureSkipVerify == 1
		l.Info("使用证书认证模式")

	case "incluster":
		l.Info("使用 InCluster 认证模式")

	default:
		l.Errorf("不支持的认证模式: %s", in.AuthType)
		return nil, errorx.Msg("不支持的认证模式")
	}

	if err := l.svcCtx.K8sManager.AddCluster(clusterConfig); err != nil {
		l.Errorf("添加临时集群失败: %v", err)
		return nil, errorx.Msg("添加临时集群失败")
	}
	l.Info("临时集群添加成功")

	// 确保函数结束时删除临时集群
	defer func() {
		l.Infof("删除临时测试集群: %s", clusterUuid)
		if err := l.svcCtx.K8sManager.RemoveCluster(clusterUuid); err != nil {
			l.Errorf("删除临时集群失败: %v", err)
		} else {
			l.Info("临时集群删除成功")
		}
	}()

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, errorx.Msg("获取集群客户端失败")
	}

	// 执行健康检查
	l.Info("开始健康检查")
	if err := client.HealthCheck(l.ctx); err != nil {
		l.Errorf("集群连接测试失败: %v", err)
		return nil, errorx.Msg(fmt.Sprintf("集群连接测试失败: %v", err))
	}

	l.Info("集群连接测试成功")
	return &pb.ClusterTestConnectResp{}, nil
}
