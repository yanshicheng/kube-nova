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

type AlertGroupsUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupsUpdateLogic {
	return &AlertGroupsUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupsUpdateLogic) AlertGroupsUpdate(in *pb.UpdateAlertGroupsReq) (*pb.UpdateAlertGroupsResp, error) {
	// 查找原有数据
	oldData, err := l.svcCtx.AlertGroupsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("告警组不存在: %v", err)
		return nil, errorx.Msg("告警组不存在")
	}

	// 更新数据
	data := &model.AlertGroups{
		Id:           in.Id,
		Uuid:         oldData.Uuid,
		GroupName:    in.GroupName,
		GroupType:    in.GroupType,
		Description:  in.Description,
		FilterRules:  in.FilterRules,
		DutySchedule: in.DutySchedule,
		SortOrder:    uint64(in.SortOrder),
		CreatedBy:    oldData.CreatedBy,
		UpdatedBy:    in.UpdatedBy,
		UpdatedAt:    time.Now(),
		IsDeleted:    oldData.IsDeleted,
	}

	err = l.svcCtx.AlertGroupsModel.Update(l.ctx, data)
	if err != nil {
		l.Errorf("更新告警组失败: %v", err)
		return nil, errorx.Msg("更新告警组失败")
	}

	return &pb.UpdateAlertGroupsResp{}, nil
}
