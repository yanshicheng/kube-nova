package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysUserStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysUserStatusLogic {
	return &UpdateSysUserStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysUserStatusLogic) UpdateSysUserStatus(req *types.UpdateSysUserStatusRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新用户状态
	_, err = l.svcCtx.PortalRpc.UserUpdateStatus(l.ctx, &pb.UpdateSysUserStatusReq{
		Id:        req.Id,
		Status:    req.Status,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("更新用户状态失败: operator=%s, userId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新用户状态成功: operator=%s, userId=%d, status=%d", username, req.Id, req.Status)
	return "更新用户状态成功", nil
}
