package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
)

type PVDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 PV
func NewPVDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVDeleteLogic {
	return &PVDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVDeleteLogic) PVDelete(req *types.ClusterResourceDeleteRequest) (resp string, err error) {
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

	// 获取 PV 详情用于审计
	existingPV, _ := pvOp.Get(req.Name)
	var capacity string
	var status string
	var storageClass string
	var claimInfo string
	var accessModes []string
	if existingPV != nil {
		if qty, ok := existingPV.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = qty.String()
		}
		status = string(existingPV.Status.Phase)
		storageClass = existingPV.Spec.StorageClassName
		if existingPV.Spec.ClaimRef != nil {
			claimInfo = fmt.Sprintf("%s/%s", existingPV.Spec.ClaimRef.Namespace, existingPV.Spec.ClaimRef.Name)
		}
		for _, mode := range existingPV.Spec.AccessModes {
			accessModes = append(accessModes, string(mode))
		}
	}

	// 获取使用情况
	usage, _ := pvOp.GetUsage(req.Name)
	var usageInfo string
	if usage != nil && len(usage.UsedByPods) > 0 {
		var podNames []string
		for _, pod := range usage.UsedByPods {
			podNames = append(podNames, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
		usageInfo = fmt.Sprintf(", 使用中的 Pod: %s", strings.Join(podNames, ", "))
	}

	err = pvOp.Delete(req.Name)
	if err != nil {
		l.Errorf("删除 PV 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 PersistentVolume",
			ActionDetail: fmt.Sprintf("用户 %s 删除 PersistentVolume %s 失败, 错误: %v", username, req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("删除 PV 失败")
	}

	l.Infof("用户: %s, 成功删除 PV: %s", username, req.Name)

	// 构建详细审计信息
	var claimDetail string
	if claimInfo != "" {
		claimDetail = fmt.Sprintf(", 绑定的 PVC: %s", claimInfo)
	}

	auditDetail := fmt.Sprintf("用户 %s 成功删除 PersistentVolume %s, 容量: %s, 状态: %s, StorageClass: %s, 访问模式: [%s]%s%s",
		username, req.Name, capacity, status, storageClass, strings.Join(accessModes, ", "), claimDetail, usageInfo)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 PersistentVolume",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "删除 PV 成功", nil
}
