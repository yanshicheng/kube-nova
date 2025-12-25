package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertRuleGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据ID获取告警规则分组详细信息
func NewGetAlertRuleGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertRuleGroupLogic {
	return &GetAlertRuleGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertRuleGroupLogic) GetAlertRuleGroup(req *types.DefaultIdRequest) (resp *types.AlertRuleGroup, err error) {
	// 调用RPC服务获取告警规则分组详情
	result, err := l.svcCtx.ManagerRpc.AlertRuleGroupGetById(l.ctx, &pb.GetAlertRuleGroupByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("获取告警规则分组详情失败: %v", err)
		return nil, fmt.Errorf("获取告警规则分组详情失败: %v", err)
	}

	if result.Data == nil {
		return nil, fmt.Errorf("告警规则分组不存在")
	}

	// 转换为API响应类型
	resp = &types.AlertRuleGroup{
		Id:          result.Data.Id,
		FileId:      result.Data.FileId,
		GroupCode:   result.Data.GroupCode,
		GroupName:   result.Data.GroupName,
		Description: result.Data.Description,
		Interval:    result.Data.Interval,
		IsEnabled:   result.Data.IsEnabled,
		SortOrder:   result.Data.SortOrder,
		RuleCount:   result.Data.RuleCount,
		FileCode:    result.Data.FileCode,
		FileName:    result.Data.FileName,
		CreatedBy:   result.Data.CreatedBy,
		UpdatedBy:   result.Data.UpdatedBy,
		CreatedAt:   result.Data.CreatedAt,
		UpdatedAt:   result.Data.UpdatedAt,
	}

	return resp, nil
}
