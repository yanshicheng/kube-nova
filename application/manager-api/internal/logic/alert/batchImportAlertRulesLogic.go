package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BatchImportAlertRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 从YAML批量导入告警规则
func NewBatchImportAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchImportAlertRulesLogic {
	return &BatchImportAlertRulesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchImportAlertRulesLogic) BatchImportAlertRules(req *types.BatchImportAlertRulesRequest) (resp *types.BatchImportAlertRulesResponse, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务批量导入告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRulesBatchImport(l.ctx, &pb.BatchImportAlertRulesReq{
		YamlStr:   req.YamlStr,
		CreatedBy: username,
		Overwrite: req.Overwrite,
	})

	if err != nil {
		l.Errorf("批量导入告警规则失败: %v", err)
		return nil, fmt.Errorf("批量导入告警规则失败: %v", err)
	}

	resp = &types.BatchImportAlertRulesResponse{
		FileId:     result.FileId,
		GroupCount: result.GroupCount,
		RuleCount:  result.RuleCount,
	}

	return resp, nil
}
