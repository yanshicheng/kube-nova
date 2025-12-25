package alertservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertGroupAppsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchAlertGroupAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertGroupAppsLogic {
	return &SearchAlertGroupAppsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchAlertGroupApps 查询告警组应用关联
func (l *SearchAlertGroupAppsLogic) SearchAlertGroupApps(in *pb.SearchAlertGroupAppsReq) (*pb.SearchAlertGroupAppsResp, error) {
	var queryStr string
	var args []interface{}
	conditions := []string{}

	// 构建查询条件
	if in.GroupId != 0 {
		conditions = append(conditions, "`group_id` = ?")
		args = append(args, in.GroupId)
	}

	if in.AppType != "" {
		conditions = append(conditions, "`app_type` = ?")
		args = append(args, in.AppType)
	}

	if len(conditions) > 0 {
		queryStr = joinConditions(conditions, " AND ")
	}

	// 执行不分页查询
	dataList, err := l.svcCtx.AlertGroupAppsModel.SearchNoPage(
		l.ctx,
		"",    // orderStr 为空，使用默认排序
		false, // 降序
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未找到告警组应用关联: groupId=%d, appType=%s", in.GroupId, in.AppType)
			return &pb.SearchAlertGroupAppsResp{}, nil
		}
		l.Errorf("查询告警组应用关联失败: %v", err)
		return nil, errorx.Msg("查询告警组应用关联失败")
	}
	if len(dataList) == 0 {
		l.Infof("未找到告警组应用关联: groupId=%d, appType=%s", in.GroupId, in.AppType)
		return &pb.SearchAlertGroupAppsResp{}, nil
	}
	return &pb.SearchAlertGroupAppsResp{
		Id:        dataList[0].Id,
		GroupId:   dataList[0].GroupId,
		AppId:     dataList[0].AppId,
		AppType:   dataList[0].AppType,
		CreatedBy: dataList[0].CreatedBy,
	}, nil
}
