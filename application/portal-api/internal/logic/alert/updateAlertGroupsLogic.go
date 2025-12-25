package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAlertGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAlertGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertGroupsLogic {
	return &UpdateAlertGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAlertGroupsLogic) UpdateAlertGroups(req *types.UpdateAlertGroupsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("更新告警组请求: operator=%s, id=%d, groupName=%s", username, req.Id, req.GroupName)

	// 调用 RPC 服务更新告警组
	_, err = l.svcCtx.AlertPortalRpc.AlertGroupsUpdate(l.ctx, &pb.UpdateAlertGroupsReq{
		Id:           req.Id,
		GroupName:    req.GroupName,
		GroupType:    req.GroupType,
		Description:  req.Description,
		FilterRules:  req.FilterRules,
		DutySchedule: req.DutySchedule,
		SortOrder:    req.SortOrder,
		UpdatedBy:    username,
	})
	if err != nil {
		l.Errorf("更新告警组失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新告警组成功: operator=%s, id=%d", username, req.Id)
	return "更新告警组成功", nil
}
