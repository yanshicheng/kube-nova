package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelAlertGroupMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelAlertGroupMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelAlertGroupMembersLogic {
	return &DelAlertGroupMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelAlertGroupMembersLogic) DelAlertGroupMembers(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("删除告警组成员请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务删除告警组成员
	_, err = l.svcCtx.AlertPortalRpc.AlertGroupMembersDel(l.ctx, &pb.DelAlertGroupMembersReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除告警组成员失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除告警组成员成功: operator=%s, id=%d", username, req.Id)
	return "删除告警组成员成功", nil
}
