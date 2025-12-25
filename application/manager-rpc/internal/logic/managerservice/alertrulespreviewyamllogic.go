package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRulesPreviewYamlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRulesPreviewYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRulesPreviewYamlLogic {
	return &AlertRulesPreviewYamlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRulesPreviewYaml 预览生成的 YAML (不实际部署)
func (l *AlertRulesPreviewYamlLogic) AlertRulesPreviewYaml(in *pb.PreviewAlertRulesYamlReq) (*pb.PreviewAlertRulesYamlResp, error) {
	// 创建部署逻辑实例，复用其生成 YAML 的能力
	deployLogic := NewAlertRulesDeployLogic(l.ctx, l.svcCtx)

	// 生成 PrometheusRule YAML（clusterUuid 传空字符串，因为只是预览）
	yamlStr, err := deployLogic.GeneratePrometheusRuleYaml(
		in.FileId,
		in.Namespace,
		in.GroupIds,
		in.RuleIds,
		"", // 预览时不需要 clusterUuid
	)
	if err != nil {
		l.Errorf("生成告警规则 YAML 失败: fileId=%d, error: %v", in.FileId, err)
		return nil, err
	}

	l.Infof("成功生成告警规则 YAML 预览: fileId=%d, namespace=%s", in.FileId, in.Namespace)

	return &pb.PreviewAlertRulesYamlResp{
		YamlStr: yamlStr,
	}, nil
}
