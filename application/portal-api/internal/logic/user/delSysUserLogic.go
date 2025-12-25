package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysUserLogic {
	return &DelSysUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysUserLogic) DelSysUser(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("删除用户请求: operator=%s, userId=%d", username, req.Id)

	// 调用 RPC 服务删除用户
	_, err = l.svcCtx.PortalRpc.UserDel(l.ctx, &pb.DelSysUserReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除用户失败: operator=%s, userId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除用户成功: operator=%s, userId=%d", username, req.Id)
	return "删除用户成功", nil
}
