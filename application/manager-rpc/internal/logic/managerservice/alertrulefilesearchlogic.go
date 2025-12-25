package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleFileSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleFileSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleFileSearchLogic {
	return &AlertRuleFileSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleFileSearch 分页搜索告警规则文件
func (l *AlertRuleFileSearchLogic) AlertRuleFileSearch(in *pb.SearchAlertRuleFileReq) (*pb.SearchAlertRuleFileResp, error) {
	var queryStr string
	var args []interface{}

	// 构建查询条件
	if in.FileCode != "" {
		queryStr = "`file_code` LIKE ?"
		args = append(args, "%"+in.FileCode+"%")
	}

	if in.FileName != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`file_name` LIKE ?"
		args = append(args, "%"+in.FileName+"%")
	}

	if in.Namespace != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`namespace` = ?"
		args = append(args, in.Namespace)
	}

	// 确定排序字段
	orderStr := "sort_order"
	if in.OrderField != "" {
		orderStr = in.OrderField
	}

	// 分页查询
	list, total, err := l.svcCtx.AlertRuleFilesModel.Search(
		l.ctx,
		orderStr,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		return nil, errorx.Msg("查询告警规则文件列表失败")
	}

	var files []*pb.AlertRuleFile
	for _, item := range list {
		// 统计关联的分组数量
		groupCount, _ := l.svcCtx.AlertRuleGroupsModel.CountByFileId(l.ctx, item.Id)
		// 统计关联的规则数量
		ruleCount, _ := l.svcCtx.AlertRulesModel.CountByFileId(l.ctx, item.Id)

		files = append(files, &pb.AlertRuleFile{
			Id:          item.Id,
			FileCode:    item.FileCode,
			FileName:    item.FileName,
			Description: item.Description,
			Namespace:   item.Namespace,
			Labels:      item.Labels.String,
			IsEnabled:   item.IsEnabled == 1,
			SortOrder:   item.SortOrder,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt.Unix(),
			UpdatedAt:   item.UpdatedAt.Unix(),
			GroupCount:  groupCount,
			RuleCount:   ruleCount,
		})
	}

	return &pb.SearchAlertRuleFileResp{
		Data:  files,
		Total: total,
	}, nil
}
