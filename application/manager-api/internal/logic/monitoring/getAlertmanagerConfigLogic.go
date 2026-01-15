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

type GetAlertmanagerConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定集群的Alertmanager配置
func NewGetAlertmanagerConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertmanagerConfigLogic {
	return &GetAlertmanagerConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertmanagerConfigLogic) GetAlertmanagerConfig(req *types.GetAlertmanagerConfigRequest) (resp *types.MonitoringConfigResponse, err error) {
	// 设置默认值
	namespace := req.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}
	crdType := req.CrdType
	if crdType == "" {
		crdType = "secret"
	}
	name := req.Name
	if name == "" {
		name = "alertmanager-main"
	}

	// 调用RPC服务获取配置
	rpcResp, err := l.svcCtx.ManagerRpc.GetAlertmanagerConfig(l.ctx, &pb.GetAlertmanagerConfigReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   namespace,
		CrdType:     crdType,
		Name:        name,
		Token:       l.svcCtx.Config.Webhook.Token,
	})
	if err != nil {
		l.Errorf("获取Alertmanager配置失败: %v", err)
		return nil, err
	}

	l.Infof("成功获取集群 [%s] 的Alertmanager配置", req.ClusterUuid)
	return &types.MonitoringConfigResponse{
		Config: rpcResp.Config,
	}, nil
}
