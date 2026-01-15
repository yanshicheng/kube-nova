package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SetAlertmanagerConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetAlertmanagerConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetAlertmanagerConfigLogic {
	return &SetAlertmanagerConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SetAlertmanagerConfig 配置Alertmanager
// 1. 调用配置生成函数生成 alertmanager.yaml 内容
// 2. 根据 CrdType 使用 Secret 或 ConfigMap Operator 创建或更新到集群
func (l *SetAlertmanagerConfigLogic) SetAlertmanagerConfig(in *pb.SetAlertmanagerConfigReq) (*pb.SetAlertmanagerConfigResp, error) {
	// 设置默认值
	namespace := in.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}
	crdType := in.CrdType
	if crdType == "" {
		crdType = "secret"
	}
	name := in.Name
	if name == "" {
		name = "alertmanager-main"
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
	// 生成 alertmanager.yaml 配置内容（不含 Secret/ConfigMap 外层）
	alertmanagerYAML := GenerateAlertmanagerConfigYAML(webhookUrl, in.Token)

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, in.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败, clusterUuid: %s, error: %v", in.ClusterUuid, err)
		return nil, errorx.Msg("查询集群失败")
	}

	// 标准标签
	labels := map[string]string{
		"app.kubernetes.io/component": "alert-router",
		"app.kubernetes.io/instance":  "main",
		"app.kubernetes.io/name":      "alertmanager",
		"app.kubernetes.io/part-of":   "kube-prometheus",
		"app.kubernetes.io/version":   "0.28.0",
	}

	// 根据资源类型创建或更新配置
	switch crdType {
	case "configmap":
		cmOperator := client.ConfigMaps()

		// 尝试获取现有 ConfigMap
		existing, getErr := cmOperator.Get(namespace, name)
		if getErr != nil {
			if errors.IsNotFound(getErr) {
				// ConfigMap 不存在，创建新的
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels:    labels,
					},
					Data: map[string]string{
						"alertmanager.yaml": alertmanagerYAML,
					},
				}
				_, createErr := cmOperator.Create(cm)
				if createErr != nil {
					l.Errorf("创建 Alertmanager ConfigMap 失败: %v", createErr)
					return nil, errorx.Msg("创建 Alertmanager 配置失败")
				}
				l.Infof("成功创建 Alertmanager ConfigMap: %s/%s", namespace, name)
			} else {
				l.Errorf("获取 ConfigMap 失败: %v", getErr)
				return nil, errorx.Msg("获取 Alertmanager 配置失败")
			}
		} else {
			// ConfigMap 存在，更新它
			existing.Data = map[string]string{
				"alertmanager.yaml": alertmanagerYAML,
			}
			if existing.Labels == nil {
				existing.Labels = make(map[string]string)
			}
			for k, v := range labels {
				existing.Labels[k] = v
			}
			_, updateErr := cmOperator.Update(existing)
			if updateErr != nil {
				l.Errorf("更新 Alertmanager ConfigMap 失败: %v", updateErr)
				return nil, errorx.Msg("更新 Alertmanager 配置失败")
			}
			l.Infof("成功更新 Alertmanager ConfigMap: %s/%s", namespace, name)
		}

	case "secret":
		secretOperator := client.Secrets()

		// 尝试获取现有 Secret
		existing, getErr := secretOperator.Get(namespace, name)
		if getErr != nil {
			if errors.IsNotFound(getErr) {
				// Secret 不存在，创建新的
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels:    labels,
					},
					Type: corev1.SecretTypeOpaque,
					StringData: map[string]string{
						"alertmanager.yaml": alertmanagerYAML,
					},
				}
				_, createErr := secretOperator.Create(secret)
				if createErr != nil {
					l.Errorf("创建 Alertmanager Secret 失败: %v", createErr)
					return nil, errorx.Msg("创建 Alertmanager 配置失败")
				}
				l.Infof("成功创建 Alertmanager Secret: %s/%s", namespace, name)
			} else {
				l.Errorf("获取 Secret 失败: %v", getErr)
				return nil, errorx.Msg("获取 Alertmanager 配置失败")
			}
		} else {
			// Secret 存在，更新它
			existing.Data = nil // 清空原有 Data，避免合并问题
			existing.StringData = map[string]string{
				"alertmanager.yaml": alertmanagerYAML,
			}
			if existing.Labels == nil {
				existing.Labels = make(map[string]string)
			}
			for k, v := range labels {
				existing.Labels[k] = v
			}
			_, updateErr := secretOperator.Update(existing)
			if updateErr != nil {
				l.Errorf("更新 Alertmanager Secret 失败: %v", updateErr)
				return nil, errorx.Msg("更新 Alertmanager 配置失败")
			}
			l.Infof("成功更新 Alertmanager Secret: %s/%s", namespace, name)
		}

	default:
		l.Errorf("不支持的资源类型: %s", crdType)
		return nil, errorx.Msg("不支持的资源类型，请使用 configmap 或 secret")
	}

	l.Infof("用户 [%s] 成功配置集群 [%s] 的 Alertmanager, namespace: %s, name: %s, type: %s",
		in.Operator, in.ClusterUuid, namespace, name, crdType)

	return &pb.SetAlertmanagerConfigResp{}, nil
}
