package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddSysRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysRoleLogic {
	return &AddSysRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysRoleLogic) AddSysRole(req *types.AddSysRoleRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务添加角色
	_, err = l.svcCtx.PortalRpc.RoleAdd(l.ctx, &pb.AddSysRoleReq{
		Name:      req.Name,
		Code:      req.Code,
		Remark:    req.Remark,
		CreatedBy: username,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("添加角色失败: operator=%s, roleName=%s, error=%v", username, req.Name, err)
		return "", err
	}

	l.Infof("添加角色成功: operator=%s, roleName=%s", username, req.Name)
	return "添加角色成功", nil
}
