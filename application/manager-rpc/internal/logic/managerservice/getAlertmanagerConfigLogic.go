package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertmanagerConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAlertmanagerConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertmanagerConfigLogic {
	return &GetAlertmanagerConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetAlertmanagerConfig 获取Alertmanager配置
// 根据 CrdType 返回对应格式的完整 YAML 配置字符串（ConfigMap 或 Secret）
func (l *GetAlertmanagerConfigLogic) GetAlertmanagerConfig(in *pb.GetAlertmanagerConfigReq) (*pb.GetAlertmanagerConfigResp, error) {
	// 设置默认值
	namespace := in.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}
	name := in.Name
	if name == "" {
		name = "alertmanager-main"
	}
	// 设置资源类型默认值，默认使用 secret（因为包含敏感的 credentials）
	crdType := in.CrdType
	if crdType == "" {
		crdType = "secret"
	}
	portalUrl, err := l.svcCtx.PortalRpc.GetKubeNovaPlatformUrl(l.ctx, &portalservice.GetPlatformUrlReq{})
	if err != nil {
		l.Errorf("获取平台地址失败: %v", err)
		return nil, errorx.Msg("获取平台地址失败")
	}
	if portalUrl.Url == "" {
		l.Errorf("获取平台地址失败")
		return nil, errorx.Msg("获取平台地址失败")
	}
	webhookUrl := fmt.Sprintf("%s/manager/v1/webhook/alerts", portalUrl.Url)

	if in.Token == "" {
		l.Errorf("Token 不能为空")
		return nil, errorx.Msg("Token 不能为空")
	}
	var config string

	// 根据资源类型生成对应格式的 YAML
	switch crdType {
	case "configmap":
		// 生成 ConfigMap 格式的完整 YAML
		config = GenerateAlertmanagerConfigMapYAML(namespace, name, webhookUrl, in.Token)
		l.Infof("生成 Alertmanager ConfigMap 配置成功, namespace: %s, name: %s", namespace, name)

	case "secret":
		// 生成 Secret 格式的完整 YAML
		config = GenerateAlertmanagerSecretYAML(namespace, name, webhookUrl, in.Token)
		l.Infof("生成 Alertmanager Secret 配置成功, namespace: %s, name: %s", namespace, name)

	default:
		l.Errorf("不支持的资源类型: %s，仅支持 configmap 或 secret", crdType)
		return nil, errorx.Msg("不支持的资源类型，请使用 configmap 或 secret")
	}

	return &pb.GetAlertmanagerConfigResp{
		Config: config,
	}, nil
}
