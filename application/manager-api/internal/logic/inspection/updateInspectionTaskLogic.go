// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateInspectionTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新巡检任务
func NewUpdateInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInspectionTaskLogic {
	return &UpdateInspectionTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateInspectionTaskLogic) UpdateInspectionTask(req *types.InspectionTaskUpdateRequest) (resp *types.InspectionTask, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionTaskUpdate(l.ctx, &pb.InspectionTaskUpdateReq{
		Id:                           req.Id,
		Name:                         req.Name,
		Description:                  req.Description,
		TemplateId:                   req.TemplateId,
		ScopeType:                    req.ScopeType,
		ClusterUuid:                  req.ClusterUuid,
		ScheduleType:                 req.ScheduleType,
		CronExpr:                     req.CronExpr,
		Enabled:                      req.Enabled,
		MaxConcurrency:               req.MaxConcurrency,
		TimeoutSec:                   req.TimeoutSec,
		ConfigJson:                   req.ConfigJson,
		PrometheusEnabled:            req.PrometheusEnabled,
		PrometheusEndpoint:           req.PrometheusEndpoint,
		PrometheusAuthEnabled:        req.PrometheusAuthEnabled,
		PrometheusAuthType:           req.PrometheusAuthType,
		PrometheusUsername:           req.PrometheusUsername,
		PrometheusPassword:           req.PrometheusPassword,
		PrometheusToken:              req.PrometheusToken,
		PrometheusTlsEnabled:         req.PrometheusTlsEnabled,
		PrometheusInsecureSkipVerify: req.PrometheusInsecureSkipVerify,
		PrometheusCaCert:             req.PrometheusCaCert,
		PrometheusClientCert:         req.PrometheusClientCert,
		PrometheusClientKey:          req.PrometheusClientKey,
		UpdatedBy:                    currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertTask(rpcResp.Data), nil
}
