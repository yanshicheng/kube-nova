package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleTreeGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleTreeGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleTreeGetLogic {
	return &AlertRuleTreeGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleTreeGet 获取完整树形结构
func (l *AlertRuleTreeGetLogic) AlertRuleTreeGet(in *pb.GetAlertRuleTreeReq) (*pb.GetAlertRuleTreeResp, error) {
	var result []*pb.AlertRuleTreeNode

	// 查询文件
	var fileQueryStr string
	var fileArgs []interface{}
	if in.FileId > 0 {
		fileQueryStr = "`id` = ?"
		fileArgs = append(fileArgs, in.FileId)
	}

	files, err := l.svcCtx.AlertRuleFilesModel.SearchNoPage(l.ctx, "sort_order", true, fileQueryStr, fileArgs...)
	if err != nil {
		return nil, errorx.Msg("查询文件列表失败")
	}

	// 遍历文件构建树
	for _, file := range files {
		fileNode := &pb.AlertRuleTreeNode{
			Id:        file.Id,
			Code:      file.FileCode,
			Name:      file.FileName,
			Type:      "file",
			IsEnabled: file.IsEnabled == 1,
			Children:  []*pb.AlertRuleTreeNode{},
		}

		// 查询该文件下的分组
		groups, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "sort_order", true, "`file_id` = ?", file.Id)
		if err != nil {
			l.Logger.Errorf("查询分组列表失败: file_id=%d, error: %v", file.Id, err)
			continue
		}

		// 遍历分组
		for _, group := range groups {
			groupNode := &pb.AlertRuleTreeNode{
				Id:        group.Id,
				Code:      group.GroupCode,
				Name:      group.GroupName,
				Type:      "group",
				IsEnabled: group.IsEnabled == 1,
				Children:  []*pb.AlertRuleTreeNode{},
			}

			// 查询该分组下的规则
			rules, err := l.svcCtx.AlertRulesModel.SearchNoPage(l.ctx, "sort_order", true, "`group_id` = ?", group.Id)
			if err != nil {
				l.Logger.Errorf("查询规则列表失败: group_id=%d, error: %v", group.Id, err)
				continue
			}

			// 遍历规则
			for _, rule := range rules {
				ruleNode := &pb.AlertRuleTreeNode{
					Id:        rule.Id,
					Code:      rule.AlertName,
					Name:      rule.RuleNameCn,
					Type:      "rule",
					IsEnabled: rule.IsEnabled == 1,
					Severity:  rule.Severity,
					Children:  nil,
				}
				groupNode.Children = append(groupNode.Children, ruleNode)
			}

			fileNode.Children = append(fileNode.Children, groupNode)
		}

		result = append(result, fileNode)
	}

	return &pb.GetAlertRuleTreeResp{
		Data: result,
	}, nil
}
