package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据ID获取告警规则详细信息
func NewGetAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertRuleLogic {
	return &GetAlertRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertRuleLogic) GetAlertRule(req *types.DefaultIdRequest) (resp *types.AlertRule, err error) {
	// 调用RPC服务获取告警规则详情
	result, err := l.svcCtx.ManagerRpc.AlertRuleGetById(l.ctx, &pb.GetAlertRuleByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("获取告警规则详情失败: %v", err)
		return nil, fmt.Errorf("获取告警规则详情失败: %v", err)
	}

	if result.Data == nil {
		return nil, fmt.Errorf("告警规则不存在")
	}

	// 转换为API响应类型
	resp = &types.AlertRule{
		Id:          result.Data.Id,
		GroupId:     result.Data.GroupId,
		AlertName:   result.Data.AlertName,
		RuleNameCn:  result.Data.RuleNameCn,
		Expr:        result.Data.Expr,
		ForDuration: result.Data.ForDuration,
		Severity:    result.Data.Severity,
		Summary:     result.Data.Summary,
		Description: result.Data.Description,
		Labels:      result.Data.Labels,
		Annotations: result.Data.Annotations,
		IsEnabled:   result.Data.IsEnabled,
		SortOrder:   result.Data.SortOrder,
		GroupCode:   result.Data.GroupCode,
		GroupName:   result.Data.GroupName,
		FileId:      result.Data.FileId,
		FileCode:    result.Data.FileCode,
		CreatedBy:   result.Data.CreatedBy,
		UpdatedBy:   result.Data.UpdatedBy,
		CreatedAt:   result.Data.CreatedAt,
		UpdatedAt:   result.Data.UpdatedAt,
	}

	return resp, nil
}
