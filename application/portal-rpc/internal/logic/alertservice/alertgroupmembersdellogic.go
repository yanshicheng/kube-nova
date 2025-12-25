package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupMembersDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupMembersDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupMembersDelLogic {
	return &AlertGroupMembersDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupMembersDelLogic) AlertGroupMembersDel(in *pb.DelAlertGroupMembersReq) (*pb.DelAlertGroupMembersResp, error) {
	err := l.svcCtx.AlertGroupMembersModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("删除告警组成员失败")
	}

	return &pb.DelAlertGroupMembersResp{}, nil
}
