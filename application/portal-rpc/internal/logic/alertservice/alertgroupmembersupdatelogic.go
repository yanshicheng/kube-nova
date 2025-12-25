package alertservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupMembersUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupMembersUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupMembersUpdateLogic {
	return &AlertGroupMembersUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupMembersUpdateLogic) AlertGroupMembersUpdate(in *pb.UpdateAlertGroupMembersReq) (*pb.UpdateAlertGroupMembersResp, error) {
	oldData, err := l.svcCtx.AlertGroupMembersModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("告警组成员不存在")
	}

	data := &model.AlertGroupMembers{
		Id:        in.Id,
		GroupId:   in.GroupId,
		UserId:    in.UserId,
		Role:      in.Role,
		CreatedBy: oldData.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		UpdatedAt: time.Now(),
		IsDeleted: oldData.IsDeleted,
	}

	err = l.svcCtx.AlertGroupMembersModel.Update(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("更新告警组成员失败")
	}

	return &pb.UpdateAlertGroupMembersResp{}, nil
}
