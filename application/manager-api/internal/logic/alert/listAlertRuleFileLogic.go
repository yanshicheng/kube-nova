package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListAlertRuleFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列表查询告警规则文件(无分页)
func NewListAlertRuleFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertRuleFileLogic {
	return &ListAlertRuleFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAlertRuleFileLogic) ListAlertRuleFile(req *types.ListAlertRuleFileRequest) (resp []types.AlertRuleFile, err error) {
	// 调用RPC服务列表查询告警规则文件
	result, err := l.svcCtx.ManagerRpc.AlertRuleFileList(l.ctx, &pb.ListAlertRuleFileReq{
		Namespace: req.Namespace,
	})

	if err != nil {
		l.Errorf("列表查询告警规则文件失败: %v", err)
		return nil, fmt.Errorf("列表查询告警规则文件失败: %v", err)
	}

	// 转换为API响应类型
	resp = make([]types.AlertRuleFile, 0, len(result.Data))
	for _, item := range result.Data {
		resp = append(resp, types.AlertRuleFile{
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

	return resp, nil
}
