package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertRuleFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索告警规则文件列表
func NewSearchAlertRuleFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertRuleFileLogic {
	return &SearchAlertRuleFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertRuleFileLogic) SearchAlertRuleFile(req *types.SearchAlertRuleFileRequest) (resp *types.SearchAlertRuleFileResponse, err error) {
	// 调用RPC服务搜索告警规则文件
	result, err := l.svcCtx.ManagerRpc.AlertRuleFileSearch(l.ctx, &pb.SearchAlertRuleFileReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		FileCode:   req.FileCode,
		FileName:   req.FileName,
		Namespace:  req.Namespace,
	})

	if err != nil {
		l.Errorf("搜索告警规则文件失败: %v", err)
		return nil, fmt.Errorf("搜索告警规则文件失败: %v", err)
	}

	// 转换为API响应类型
	items := make([]types.AlertRuleFile, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertRuleFile{
			Id:          item.Id,
			FileCode:    item.FileCode,
			FileName:    item.FileName,
			Description: item.Description,
			Namespace:   item.Namespace,
			Labels:      item.Labels,
			IsEnabled:   item.IsEnabled,
			SortOrder:   item.SortOrder,
			GroupCount:  item.GroupCount,
			RuleCount:   item.RuleCount,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertRuleFileResponse{
		Items: items,
		Total: result.Total,
	}

	return resp, nil
}
