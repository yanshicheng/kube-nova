package alertservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupsSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupsSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupsSearchLogic {
	return &AlertGroupsSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupsSearchLogic) AlertGroupsSearch(in *pb.SearchAlertGroupsReq) (*pb.SearchAlertGroupsResp, error) {
	// 构建查询条件
	var queryStr string
	var args []interface{}

	conditions := []string{}

	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}

	if in.GroupName != "" {
		conditions = append(conditions, "`group_name` LIKE ?")
		args = append(args, "%"+in.GroupName+"%")
	}

	if in.GroupType != "" {
		conditions = append(conditions, "`group_type` = ?")
		args = append(args, in.GroupType)
	}

	if len(conditions) > 0 {
		queryStr = fmt.Sprintf("%s", joinConditions(conditions, " AND "))
	}

	// 执行搜索
	dataList, total, err := l.svcCtx.AlertGroupsModel.Search(
		l.ctx,
		in.OrderField,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.SearchAlertGroupsResp{}, nil
		}
		return nil, errorx.Msg("搜索告警组失败")
	}

	// 转换数据
	list := make([]*pb.AlertGroups, 0, len(dataList))
	for _, data := range dataList {
		list = append(list, &pb.AlertGroups{
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
		})
	}

	return &pb.SearchAlertGroupsResp{
		Data:  list,
		Total: total,
	}, nil
}
