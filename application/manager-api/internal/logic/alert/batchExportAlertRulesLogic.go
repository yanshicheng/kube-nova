package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BatchExportAlertRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 导出告警规则为YAML
func NewBatchExportAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchExportAlertRulesLogic {
	return &BatchExportAlertRulesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchExportAlertRulesLogic) BatchExportAlertRules(req *types.BatchExportAlertRulesRequest) (resp *types.BatchExportAlertRulesResponse, err error) {
	// 调用RPC服务批量导出告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRulesBatchExport(l.ctx, &pb.BatchExportAlertRulesReq{
		FileId:   req.FileId,
		GroupIds: req.GroupIds,
	})

	if err != nil {
		l.Errorf("批量导出告警规则失败: %v", err)
		return nil, fmt.Errorf("批量导出告警规则失败: %v", err)
	}

	resp = &types.BatchExportAlertRulesResponse{
		YamlStr: result.YamlStr,
	}

	return resp, nil
}
