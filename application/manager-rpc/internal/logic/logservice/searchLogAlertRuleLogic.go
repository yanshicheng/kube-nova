package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchLogAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchLogAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchLogAlertRuleLogic {
	return &SearchLogAlertRuleLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *SearchLogAlertRuleLogic) SearchLogAlertRule(in *pb.SearchLogAlertRuleReq) (*pb.SearchLogAlertRuleResp, error) {
	queryStr := ""
	args := make([]any, 0)
	appendCond := func(cond string, arg ...any) {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += cond
		args = append(args, arg...)
	}
	if in.ClusterUuid != "" {
		appendCond("`cluster_uuid` = ?", in.ClusterUuid)
	}
	if in.WorkspaceId > 0 {
		appendCond("`workspace_id` = ?", in.WorkspaceId)
	}
	if in.ProjectUuid != "" {
		appendCond("`project_uuid` = ?", in.ProjectUuid)
	}
	if in.Namespace != "" {
		appendCond("`namespace` = ?", in.Namespace)
	}
	if in.Application != "" {
		appendCond("`application` = ?", in.Application)
	}
	if in.ResourceName != "" {
		appendCond("`resource_name` = ?", in.ResourceName)
	}
	if in.Enabled == 0 || in.Enabled == 1 {
		appendCond("`enabled` = ?", in.Enabled)
	}
	orderField := in.OrderField
	if orderField == "" {
		orderField = "id"
	}
	list, total, err := l.svcCtx.OnecLogAlertRuleModel.Search(l.ctx, orderField, in.IsAsc, in.Page, in.PageSize, queryStr, args...)
	if err != nil {
		return nil, errorx.Msg("查询日志告警失败")
	}
	resp := &pb.SearchLogAlertRuleResp{Data: make([]*pb.LogAlertRule, 0, len(list)), Total: total}
	for _, item := range list {
		resp.Data = append(resp.Data, convertLogAlertRuleToPB(item))
	}
	return resp, nil
}
