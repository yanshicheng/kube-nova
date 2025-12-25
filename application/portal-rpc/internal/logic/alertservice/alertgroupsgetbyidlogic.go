package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupsGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupsGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupsGetByIdLogic {
	return &AlertGroupsGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupsGetByIdLogic) AlertGroupsGetById(in *pb.GetAlertGroupsByIdReq) (*pb.GetAlertGroupsByIdResp, error) {
	data, err := l.svcCtx.AlertGroupsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("告警组不存在")
	}

	return &pb.GetAlertGroupsByIdResp{
		Data: &pb.AlertGroups{
			Id:           data.Id,
			Uuid:         data.Uuid,
			GroupName:    data.GroupName,
			GroupType:    data.GroupType,
			Description:  data.Description,
			FilterRules:  data.FilterRules,
			DutySchedule: data.DutySchedule,
			SortOrder:    int64(data.SortOrder),
			CreatedBy:    data.CreatedBy,
			UpdatedBy:    data.UpdatedBy,
			CreatedAt:    data.CreatedAt.Unix(),
			UpdatedAt:    data.UpdatedAt.Unix(),
		},
	}, nil
}
