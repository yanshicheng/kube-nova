package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type DelLogAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDelLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelLogAlertRuleLogic {
	return &DelLogAlertRuleLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DelLogAlertRuleLogic) DelLogAlertRule(in *pb.DelLogAlertRuleReq) (*pb.DelLogAlertRuleResp, error) {
	item, err := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("日志告警不存在")
	}
	if isPlatformManagedLogBackend(l.svcCtx, item.BackendType) {
		_ = deletePlatformEvalTask(l.ctx, l.svcCtx, in.Id)
		if err := l.svcCtx.OnecLogAlertRuleModel.DeleteSoft(l.ctx, in.Id); err != nil {
			return nil, errorx.Msg("删除日志告警失败")
		}
		return &pb.DelLogAlertRuleResp{}, nil
	}
	if apps, appErr := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ? AND `is_default` = 1", item.ClusterUuid, 2); appErr == nil && len(apps) > 0 {
		if client, buildErr := buildLogClient(l.ctx, apps[0], l.svcCtx.Config.LogSearch); buildErr == nil && item.BackendRuleId != "" {
			_ = client.DeleteAlert(item.BackendRuleId)
		}
	}
	if err := l.svcCtx.OnecLogAlertRuleModel.DeleteSoft(l.ctx, in.Id); err != nil {
		return nil, errorx.Msg("删除日志告警失败")
	}
	return &pb.DelLogAlertRuleResp{}, nil
}
