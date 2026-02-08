package core

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

type PVCDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPVCDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCDeleteLogic {
	return &PVCDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCDeleteLogic) PVCDelete(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 PVC operator
	pvcOp := client.PVC()

	// 获取 PVC 详情用于审计
	existingPVC, _ := pvcOp.Get(req.Namespace, req.Name)
	var storageClass string
	var capacity string
	var accessModes string
	var volumeName string
	var status string
	if existingPVC != nil {
		if existingPVC.Spec.StorageClassName != nil {
			storageClass = *existingPVC.Spec.StorageClassName
		}
		if existingPVC.Status.Capacity != nil {
			if s, ok := existingPVC.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = s.String()
			}
		}
		accessModes = l.buildAccessModesInfo(existingPVC.Spec.AccessModes)
		volumeName = existingPVC.Spec.VolumeName
		status = string(existingPVC.Status.Phase)
	}

	// 删除 PVC
	deleteErr := pvcOp.Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 PVC 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 PVC",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 PVC %s 失败, 状态: %s, 错误原因: %v", username, req.Namespace, req.Name, status, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 PVC 失败")
	}

	// 记录成功的审计日志
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 PVC %s, 存储类: %s, 容量: %s, 访问模式: %s, 绑定PV: %s, 删除前状态: %s", username, req.Namespace, req.Name, storageClass, capacity, accessModes, volumeName, status)

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 PVC",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功删除 PVC: %s/%s", username, req.Namespace, req.Name)
	return "删除 PVC 成功", nil
}

// buildAccessModesInfo 构建访问模式信息
func (l *PVCDeleteLogic) buildAccessModesInfo(modes []corev1.PersistentVolumeAccessMode) string {
	if len(modes) == 0 {
		return "无"
	}
	var modeStrs []string
	for _, m := range modes {
		modeStrs = append(modeStrs, string(m))
	}
	return strings.Join(modeStrs, ", ")
}
