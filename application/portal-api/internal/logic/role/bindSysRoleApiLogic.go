package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindSysRoleApiLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindSysRoleApiLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindSysRoleApiLogic {
	return &BindSysRoleApiLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BindSysRoleApiLogic) BindSysRoleApi(req *types.BindSysRoleApiRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务绑定角色API权限
	_, err = l.svcCtx.PortalRpc.RoleAddApi(l.ctx, &pb.AddSysRoleApiReq{
		RoleId: req.RoleId,
		ApiIds: req.ApiIds,
	})
	if err != nil {
		l.Errorf("角色绑定API权限失败: operator=%s, roleId=%d, error=%v", username, req.RoleId, err)
		return "", err
	}

	l.Infof("角色绑定API权限成功: operator=%s, roleId=%d", username, req.RoleId)
	return "角色绑定API权限成功", nil
}
