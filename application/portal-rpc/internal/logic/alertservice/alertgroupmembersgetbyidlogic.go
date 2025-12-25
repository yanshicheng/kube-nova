package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupMembersGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupMembersGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupMembersGetByIdLogic {
	return &AlertGroupMembersGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupMembersGetByIdLogic) AlertGroupMembersGetById(in *pb.GetAlertGroupMembersByIdReq) (*pb.GetAlertGroupMembersByIdResp, error) {
	data, err := l.svcCtx.AlertGroupMembersModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("告警组成员不存在")
	}

	return &pb.GetAlertGroupMembersByIdResp{
		Data: &pb.AlertGroupMembers{
			Id:        data.Id,
			GroupId:   data.GroupId,
			UserId:    data.UserId,
			Role:      data.Role,
			CreatedBy: data.CreatedBy,
			UpdatedBy: data.UpdatedBy,
			CreatedAt: data.CreatedAt.Unix(),
			UpdatedAt: data.UpdatedAt.Unix(),
		},
	}, nil
}
