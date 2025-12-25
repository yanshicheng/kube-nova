package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type PreviewAlertRulesYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览生成的告警规则YAML
func NewPreviewAlertRulesYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreviewAlertRulesYamlLogic {
	return &PreviewAlertRulesYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PreviewAlertRulesYamlLogic) PreviewAlertRulesYaml(req *types.PreviewAlertRulesYamlRequest) (resp *types.PreviewAlertRulesYamlResponse, err error) {
	// 调用RPC服务预览生成的YAML
	result, err := l.svcCtx.ManagerRpc.AlertRulesPreviewYaml(l.ctx, &pb.PreviewAlertRulesYamlReq{
		FileId:    req.FileId,
		Namespace: req.Namespace,
		GroupIds:  req.GroupIds,
		RuleIds:   req.RuleIds,
	})

	if err != nil {
		l.Errorf("预览告警规则YAML失败: %v", err)
		return nil, fmt.Errorf("预览告警规则YAML失败: %v", err)
	}

	resp = &types.PreviewAlertRulesYamlResponse{
		YamlStr: result.YamlStr,
	}

	return resp, nil
}
