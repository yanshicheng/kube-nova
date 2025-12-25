package managerservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleSearchLogic {
	return &AlertRuleSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleSearch 分页搜索告警规则
func (l *AlertRuleSearchLogic) AlertRuleSearch(in *pb.SearchAlertRuleReq) (*pb.SearchAlertRuleResp, error) {
	var queryStr string
	var args []interface{}

	// 按文件筛选：需要先查询该文件下的所有分组ID
	if in.FileId > 0 {
		groups, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "", true, "`file_id` = ?", in.FileId)
		if err != nil {
			l.Logger.Errorf("查询分组失败: %v", err)
		}

		if len(groups) == 0 {
			// 该文件下没有分组，返回空结果
			return &pb.SearchAlertRuleResp{Data: nil, Total: 0}, nil
		}

		groupIds := make([]string, len(groups))
		for i, g := range groups {
			groupIds[i] = fmt.Sprintf("%d", g.Id)
		}
		queryStr = fmt.Sprintf("`group_id` IN (%s)", strings.Join(groupIds, ","))
	}

	// 按分组筛选
	if in.GroupId > 0 {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`group_id` = ?"
		args = append(args, in.GroupId)
	}

	// 告警名称模糊搜索
	if in.AlertName != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`alert_name` LIKE ?"
		args = append(args, "%"+in.AlertName+"%")
	}

	// 中文名模糊搜索
	if in.RuleNameCn != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`rule_name_cn` LIKE ?"
		args = append(args, "%"+in.RuleNameCn+"%")
	}

	// 按级别筛选
	if in.Severity != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "`severity` = ?"
		args = append(args, in.Severity)
	}

	// 确定排序字段
	orderStr := "sort_order"
	if in.OrderField != "" {
		orderStr = in.OrderField
	}

	// 分页查询
	list, total, err := l.svcCtx.AlertRulesModel.Search(
		l.ctx,
		orderStr,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		return nil, errorx.Msg("查询告警规则列表失败")
	}

	// 预加载分组和文件信息（避免 N+1 查询）
	groupMap := make(map[uint64]*model.AlertRuleGroups)
	fileMap := make(map[uint64]*model.AlertRuleFiles)
	for _, item := range list {
		if _, ok := groupMap[item.GroupId]; !ok {
			group, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, item.GroupId)
			if err == nil {
				groupMap[item.GroupId] = group
				if _, ok := fileMap[group.FileId]; !ok {
					file, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, group.FileId)
					if err == nil {
						fileMap[group.FileId] = file
					}
				}
			}
		}
	}

	var rules []*pb.AlertRule
	for _, item := range list {
		var groupCode, groupName string
		var fileId uint64
		var fileCode string

		if group, ok := groupMap[item.GroupId]; ok {
			groupCode = group.GroupCode
			groupName = group.GroupName
			fileId = group.FileId
			if file, ok := fileMap[group.FileId]; ok {
				fileCode = file.FileCode
			}
		}

		rules = append(rules, &pb.AlertRule{
			Id:          item.Id,
			GroupId:     item.GroupId,
			AlertName:   item.AlertName,
			RuleNameCn:  item.RuleNameCn,
			Expr:        item.Expr,
			ForDuration: item.ForDuration,
			Severity:    item.Severity,
			Summary:     item.Summary,
			Description: item.Description.String,
			Labels:      item.Labels.String,
			Annotations: item.Annotations.String,
			IsEnabled:   item.IsEnabled == 1,
			SortOrder:   item.SortOrder,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt.Unix(),
			UpdatedAt:   item.UpdatedAt.Unix(),
			GroupCode:   groupCode,
			GroupName:   groupName,
			FileId:      fileId,
			FileCode:    fileCode,
		})
	}

	return &pb.SearchAlertRuleResp{
		Data:  rules,
		Total: total,
	}, nil
}
