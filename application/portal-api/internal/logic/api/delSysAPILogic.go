package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysAPILogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysAPILogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysAPILogic {
	return &DelSysAPILogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysAPILogic) DelSysAPI(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除API
	_, err = l.svcCtx.PortalRpc.APIDel(l.ctx, &pb.DelSysAPIReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除API失败: operator=%s, apiId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	return "删除API成功", nil
}
