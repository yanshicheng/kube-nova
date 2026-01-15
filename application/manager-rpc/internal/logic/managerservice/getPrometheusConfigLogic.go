package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrometheusConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPrometheusConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrometheusConfigLogic {
	return &GetPrometheusConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPrometheusConfig 获取Prometheus配置
// 直接调用内置的配置生成函数，返回完整的 Prometheus CRD YAML 配置字符串
func (l *GetPrometheusConfigLogic) GetPrometheusConfig(in *pb.GetPrometheusConfigReq) (*pb.GetPrometheusConfigResp, error) {
	// 设置默认值
	namespace := in.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}
	name := in.Name
	if name == "" {
		name = "k8s"
	}

	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		l.Errorf("查询集群信息失败: %v", err)
		return nil, errorx.Msg("查询集群信息失败")
	}
	labels := PrometheusExternalLabels{
		Environment: cluster.Environment,
		Cluster:     cluster.Name,
		ClusterUuid: cluster.Uuid,
		Region:      cluster.Region,
		Datacenter:  cluster.Datacenter,
	}

	// 调用配置生成函数，生成完整的 Prometheus CRD YAML
	config := GeneratePrometheusConfigYAML(namespace, name, labels)

	l.Infof("生成 Prometheus 配置成功, namespace: %s, name: %s, clusterUuid: %s", namespace, name, in.ClusterUuid)

	return &pb.GetPrometheusConfigResp{
		Config: config,
	}, nil
}
