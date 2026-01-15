// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package monitoring

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrometheusConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定集群的Prometheus配置
func NewGetPrometheusConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrometheusConfigLogic {
	return &GetPrometheusConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPrometheusConfigLogic) GetPrometheusConfig(req *types.GetPrometheusConfigRequest) (resp *types.MonitoringConfigResponse, err error) {
	// 设置默认值
	namespace := req.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}
	name := req.Name
	if name == "" {
		name = "k8s"
	}

	// 调用RPC服务获取配置
	rpcResp, err := l.svcCtx.ManagerRpc.GetPrometheusConfig(l.ctx, &pb.GetPrometheusConfigReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   namespace,
		Name:        name,
	})
	if err != nil {
		l.Errorf("获取Prometheus配置失败: %v", err)
		return nil, err
	}

	l.Infof("成功获取集群 [%s] 的Prometheus配置", req.ClusterUuid)
	return &types.MonitoringConfigResponse{
		Config: rpcResp.Config,
	}, nil
}
