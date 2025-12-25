package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysRoleLogic {
	return &UpdateSysRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysRoleLogic) UpdateSysRole(req *types.UpdateSysRoleRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新角色
	_, err = l.svcCtx.PortalRpc.RoleUpdate(l.ctx, &pb.UpdateSysRoleReq{
		Id:        req.Id,
		Name:      req.Name,
		Code:      req.Code,
		Remark:    req.Remark,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("更新角色失败: operator=%s, roleId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	return "更新角色成功", nil
}
