package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAlertGroupMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAlertGroupMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertGroupMembersLogic {
	return &UpdateAlertGroupMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAlertGroupMembersLogic) UpdateAlertGroupMembers(req *types.UpdateAlertGroupMembersRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("更新告警组成员请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务更新告警组成员
	_, err = l.svcCtx.AlertPortalRpc.AlertGroupMembersUpdate(l.ctx, &pb.UpdateAlertGroupMembersReq{
		Id:        req.Id,
		GroupId:   req.GroupId,
		UserId:    req.UserId,
		Role:      req.Role,
		CreatedBy: username,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("更新告警组成员失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新告警组成员成功: operator=%s, id=%d", username, req.Id)
	return "更新告警组成员成功", nil
}
