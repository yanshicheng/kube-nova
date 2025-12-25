package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleGroupListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleGroupListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleGroupListLogic {
	return &AlertRuleGroupListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleGroupList 无分页查询告警规则分组列表（用于下拉选择）
func (l *AlertRuleGroupListLogic) AlertRuleGroupList(in *pb.ListAlertRuleGroupReq) (*pb.ListAlertRuleGroupResp, error) {
	var queryStr string
	var args []interface{}

	if in.FileId > 0 {
		queryStr = "`file_id` = ?"
		args = append(args, in.FileId)
	}

	list, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(
		l.ctx,
		"sort_order",
		true,
		queryStr,
		args...,
	)
	if err != nil {
		return nil, errorx.Msg("查询告警规则分组列表失败")
	}

	var groups []*pb.AlertRuleGroup
	for _, item := range list {
		groups = append(groups, &pb.AlertRuleGroup{
			Id:        item.Id,
			FileId:    item.FileId,
			GroupCode: item.GroupCode,
			GroupName: item.GroupName,
			Interval:  item.Interval,
			IsEnabled: item.IsEnabled == 1,
			SortOrder: item.SortOrder,
		})
	}

	return &pb.ListAlertRuleGroupResp{
		Data: groups,
	}, nil
}
