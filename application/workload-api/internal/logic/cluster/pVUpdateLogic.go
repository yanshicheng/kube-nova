package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 PV
func NewPVUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVUpdateLogic {
	return &PVUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVUpdateLogic) PVUpdate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()

	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal([]byte(req.YamlStr), &pv); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 获取旧的 PV 用于对比
	oldPV, err := pvOp.Get(pv.Name)
	if err != nil {
		l.Errorf("获取原 PV 失败: %v", err)
		return "", fmt.Errorf("获取原 PV 失败")
	}

	// 提取容量信息
	capacity := ""
	if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = qty.String()
	}

	oldCapacity := ""
	if qty, ok := oldPV.Spec.Capacity[corev1.ResourceStorage]; ok {
		oldCapacity = qty.String()
	}

	// 注入注解
	utils.AddAnnotations(&pv.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: pv.Name,
	})

	// 对比标签变更
	labelDiff := CompareStringMaps(oldPV.Labels, pv.Labels)
	labelDiffDetail := ""
	if HasMapChanges(labelDiff) {
		labelDiffDetail = "标签变更: " + BuildMapDiffDetail(labelDiff, true)
	}

	// 对比注解变更
	annotationDiff := CompareStringMaps(oldPV.Annotations, pv.Annotations)
	annotationDiffDetail := ""
	if HasMapChanges(annotationDiff) {
		annotationDiffDetail = "注解变更: " + BuildMapDiffDetail(annotationDiff, false)
	}

	// 检查容量变更
	var capacityChange string
	if oldCapacity != capacity {
		capacityChange = fmt.Sprintf("容量变更: %s → %s", oldCapacity, capacity)
	}

	// 检查回收策略变更
	oldReclaimPolicy := string(oldPV.Spec.PersistentVolumeReclaimPolicy)
	newReclaimPolicy := string(pv.Spec.PersistentVolumeReclaimPolicy)
	var reclaimPolicyChange string
	if oldReclaimPolicy != newReclaimPolicy {
		reclaimPolicyChange = fmt.Sprintf("回收策略变更: %s → %s", oldReclaimPolicy, newReclaimPolicy)
	}

	_, err = pvOp.Update(&pv)
	if err != nil {
		l.Errorf("更新 PV 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 PersistentVolume",
			ActionDetail: fmt.Sprintf("用户 %s 更新 PersistentVolume %s 失败, 错误: %v", username, pv.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("更新 PV 失败")
	}

	l.Infof("用户: %s, 成功更新 PV: %s", username, pv.Name)

	// 构建详细审计信息
	var changeParts []string
	changeParts = append(changeParts, fmt.Sprintf("用户 %s 成功更新 PersistentVolume %s", username, pv.Name))
	changeParts = append(changeParts, fmt.Sprintf("容量: %s", capacity))
	changeParts = append(changeParts, fmt.Sprintf("StorageClass: %s", pv.Spec.StorageClassName))

	if capacityChange != "" {
		changeParts = append(changeParts, capacityChange)
	}
	if reclaimPolicyChange != "" {
		changeParts = append(changeParts, reclaimPolicyChange)
	}
	if labelDiffDetail != "" {
		changeParts = append(changeParts, labelDiffDetail)
	}
	if annotationDiffDetail != "" {
		changeParts = append(changeParts, annotationDiffDetail)
	}

	auditDetail := joinNonEmpty(", ", changeParts...)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 PersistentVolume",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "更新 PV 成功", nil
}
