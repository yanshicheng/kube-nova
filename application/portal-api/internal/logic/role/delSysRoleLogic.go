package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysRoleLogic {
	return &DelSysRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysRoleLogic) DelSysRole(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除角色
	_, err = l.svcCtx.PortalRpc.RoleDel(l.ctx, &pb.DelSysRoleReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除角色失败: operator=%s, roleId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除角色成功: operator=%s, roleId=%d", username, req.Id)
	return "删除角色成功", nil
}
