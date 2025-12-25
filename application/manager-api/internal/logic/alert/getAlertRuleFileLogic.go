package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertRuleFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据ID获取告警规则文件详细信息
func NewGetAlertRuleFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertRuleFileLogic {
	return &GetAlertRuleFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertRuleFileLogic) GetAlertRuleFile(req *types.DefaultIdRequest) (resp *types.AlertRuleFile, err error) {
	// 调用RPC服务获取告警规则文件详情
	result, err := l.svcCtx.ManagerRpc.AlertRuleFileGetById(l.ctx, &pb.GetAlertRuleFileByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("获取告警规则文件详情失败: %v", err)
		return nil, fmt.Errorf("获取告警规则文件详情失败: %v", err)
	}

	if result.Data == nil {
		return nil, fmt.Errorf("告警规则文件不存在")
	}

	// 转换为API响应类型
	resp = &types.AlertRuleFile{
		Id:          result.Data.Id,
		FileCode:    result.Data.FileCode,
		FileName:    result.Data.FileName,
		Description: result.Data.Description,
		Namespace:   result.Data.Namespace,
		Labels:      result.Data.Labels,
		IsEnabled:   result.Data.IsEnabled,
		SortOrder:   result.Data.SortOrder,
		GroupCount:  result.Data.GroupCount,
		RuleCount:   result.Data.RuleCount,
		CreatedBy:   result.Data.CreatedBy,
		UpdatedBy:   result.Data.UpdatedBy,
		CreatedAt:   result.Data.CreatedAt,
		UpdatedAt:   result.Data.UpdatedAt,
	}

	return resp, nil
}
