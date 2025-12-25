package loginLog

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysLoginLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysLoginLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysLoginLogLogic {
	return &DelSysLoginLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysLoginLogLogic) DelSysLoginLog(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除登录日志
	_, err = l.svcCtx.PortalRpc.LoginLogDel(l.ctx, &pb.DelSysLoginLogReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除登录日志失败: operator=%s, logId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除登录日志成功: operator=%s, logId=%d", username, req.Id)
	return "删除登录日志成功", nil
}
