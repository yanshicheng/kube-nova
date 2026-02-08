package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 IngressClass
func NewIngressClassUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassUpdateLogic {
	return &IngressClassUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassUpdateLogic) IngressClassUpdate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()

	var ic networkingv1.IngressClass
	if err := yaml.Unmarshal([]byte(req.YamlStr), &ic); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 获取旧的 IngressClass 用于对比
	oldIC, err := icOp.Get(ic.Name)
	if err != nil {
		l.Errorf("获取原 IngressClass 失败: %v", err)
		return "", fmt.Errorf("获取原 IngressClass 失败")
	}

	// 注入注解
	utils.AddAnnotations(&ic.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: ic.Name,
	})

	// 对比标签变更
	labelDiff := CompareStringMaps(oldIC.Labels, ic.Labels)
	labelDiffDetail := ""
	if HasMapChanges(labelDiff) {
		labelDiffDetail = "标签变更: " + BuildMapDiffDetail(labelDiff, true)
	}

	// 对比注解变更
	annotationDiff := CompareStringMaps(oldIC.Annotations, ic.Annotations)
	annotationDiffDetail := ""
	if HasMapChanges(annotationDiff) {
		annotationDiffDetail = "注解变更: " + BuildMapDiffDetail(annotationDiff, false)
	}

	// 检查默认状态变更
	oldIsDefault := oldIC.Annotations != nil && oldIC.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true"
	newIsDefault := ic.Annotations != nil && ic.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true"
	var defaultChange string
	if oldIsDefault != newIsDefault {
		if newIsDefault {
			defaultChange = "默认状态: 否 → 是"
		} else {
			defaultChange = "默认状态: 是 → 否"
		}
	}

	_, err = icOp.Update(&ic)
	if err != nil {
		l.Errorf("更新 IngressClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 IngressClass",
			ActionDetail: fmt.Sprintf("用户 %s 更新 IngressClass %s 失败, 错误: %v", username, ic.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("更新 IngressClass 失败")
	}

	l.Infof("用户: %s, 成功更新 IngressClass: %s", username, ic.Name)

	// 构建详细审计信息
	var changeParts []string
	changeParts = append(changeParts, fmt.Sprintf("用户 %s 成功更新 IngressClass %s", username, ic.Name))
	changeParts = append(changeParts, fmt.Sprintf("控制器: %s", ic.Spec.Controller))

	if labelDiffDetail != "" {
		changeParts = append(changeParts, labelDiffDetail)
	}
	if annotationDiffDetail != "" {
		changeParts = append(changeParts, annotationDiffDetail)
	}
	if defaultChange != "" {
		changeParts = append(changeParts, defaultChange)
	}

	auditDetail := joinNonEmpty(", ", changeParts...)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 IngressClass",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "更新 IngressClass 成功", nil
}
