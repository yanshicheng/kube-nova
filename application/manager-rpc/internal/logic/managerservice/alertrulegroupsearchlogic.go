package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleGroupSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleGroupSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleGroupSearchLogic {
	return &AlertRuleGroupSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleGroupSearch 分页搜索告警规则分组
func (l *AlertRuleGroupSearchLogic) AlertRuleGroupSearch(in *pb.SearchAlertRuleGroupReq) (*pb.SearchAlertRuleGroupResp, error) {
	var queryStr string
	var args []interface{}

	// 构建查询条件
	if in.FileId > 0 {
		queryStr = "`file_id` = ?"
		args = append(args, in.FileId)
	}

	if in.GroupCode != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`group_code` LIKE ?"
		args = append(args, "%"+in.GroupCode+"%")
	}

	if in.GroupName != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`group_name` LIKE ?"
		args = append(args, "%"+in.GroupName+"%")
	}

	// 确定排序字段
	orderStr := "sort_order"
	if in.OrderField != "" {
		orderStr = in.OrderField
	}

	// 分页查询
	list, total, err := l.svcCtx.AlertRuleGroupsModel.Search(
		l.ctx,
		orderStr,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		return nil, errorx.Msg("查询告警规则分组列表失败")
	}

	// 预加载文件信息（避免 N+1 查询）
	fileMap := make(map[uint64]*model.AlertRuleFiles)
	for _, item := range list {
		if _, ok := fileMap[item.FileId]; !ok {
			file, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, item.FileId)
			if err == nil {
				fileMap[item.FileId] = file
			}
		}
	}

	var groups []*pb.AlertRuleGroup
	for _, item := range list {
		// 统计关联的规则数量
		ruleCount, _ := l.svcCtx.AlertRulesModel.CountByGroupId(l.ctx, item.Id)

		// 获取文件信息
		var fileCode, fileName string
		if file, ok := fileMap[item.FileId]; ok {
			fileCode = file.FileCode
			fileName = file.FileName
		}

		groups = append(groups, &pb.AlertRuleGroup{
			Id:          item.Id,
			FileId:      item.FileId,
			GroupCode:   item.GroupCode,
			GroupName:   item.GroupName,
			Description: item.Description,
			Interval:    item.Interval,
			IsEnabled:   item.IsEnabled == 1,
			SortOrder:   item.SortOrder,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt.Unix(),
			UpdatedAt:   item.UpdatedAt.Unix(),
			RuleCount:   ruleCount,
			FileCode:    fileCode,
			FileName:    fileName,
		})
	}

	return &pb.SearchAlertRuleGroupResp{
		Data:  groups,
		Total: total,
	}, nil
}
