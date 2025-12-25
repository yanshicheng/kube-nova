package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertGroupMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddAlertGroupMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertGroupMembersLogic {
	return &AddAlertGroupMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertGroupMembersLogic) AddAlertGroupMembers(req *types.AddAlertGroupMembersRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("添加告警组成员请求: operator=%s, groupId=%d, userId=%d", username, req.GroupId, req.UserId)

	// 调用 RPC 服务添加告警组成员
	_, err = l.svcCtx.AlertPortalRpc.AlertGroupMembersAdd(l.ctx, &pb.AddAlertGroupMembersReq{
		GroupId:   req.GroupId,
		UserId:    req.UserId,
		Role:      req.Role,
		CreatedBy: username,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("添加告警组成员失败: operator=%s, groupId=%d, userId=%d, error=%v", username, req.GroupId, req.UserId, err)
		return "", err
	}

	l.Infof("添加告警组成员成功: operator=%s, groupId=%d, userId=%d", username, req.GroupId, req.UserId)
	return "添加告警组成员成功", nil
}
