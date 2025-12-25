package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysUserBindRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysUserBindRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysUserBindRoleLogic {
	return &UpdateSysUserBindRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysUserBindRoleLogic) UpdateSysUserBindRole(req *types.UpdateSysUserBindRoleRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务绑定用户角色
	_, err = l.svcCtx.PortalRpc.UserUpdateBindRole(l.ctx, &pb.UpdateSysUserBindRoleReq{
		Id:      req.Id,
		RoleIds: req.RoleIds,
	})
	if err != nil {
		l.Errorf("用户绑定角色失败: operator=%s, userId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	return "用户绑定角色成功", nil
}
