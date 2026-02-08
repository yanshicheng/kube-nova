package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVCCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePVCLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCCreateLogic {
	return &PVCCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCCreateLogic) PVCCreate(req *types.DefaultCoreCreateRequest) (resp string, err error) {
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

	// 解析 YAML
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal([]byte(req.YamlStr), &pvc); err != nil {
		l.Errorf("解析 PVC YAML 失败: %v", err)
		return "", fmt.Errorf("解析 PVC YAML 失败")
	}

	// 确保命名空间正确
	if pvc.Namespace == "" {
		pvc.Namespace = req.Namespace
	}

	// 获取项目详情并注入注解
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		utils.AddAnnotations(&pvc.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   pvc.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 获取存储类和请求大小用于审计详情
	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}
	requestStorage := ""
	if pvc.Spec.Resources.Requests != nil {
		if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			requestStorage = storage.String()
		}
	}
	accessModes := l.buildAccessModesInfo(pvc.Spec.AccessModes)
	volumeMode := "Filesystem"
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	}

	// 创建 PVC
	createErr := pvcOp.Create(&pvc)
	if createErr != nil {
		l.Errorf("创建 PVC 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 PVC",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 PVC %s 失败, 存储类: %s, 请求容量: %s, 访问模式: %s, 卷模式: %s, 错误原因: %v", username, pvc.Namespace, pvc.Name, storageClass, requestStorage, accessModes, volumeMode, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 PVC 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 PVC",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 PVC %s, 存储类: %s, 请求容量: %s, 访问模式: %s, 卷模式: %s", username, pvc.Namespace, pvc.Name, storageClass, requestStorage, accessModes, volumeMode),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功创建 PVC: %s/%s", username, pvc.Namespace, pvc.Name)
	return "创建 PVC 成功", nil
}

// buildAccessModesInfo 构建访问模式信息
func (l *PVCCreateLogic) buildAccessModesInfo(modes []corev1.PersistentVolumeAccessMode) string {
	if len(modes) == 0 {
		return "无"
	}
	var modeStrs []string
	for _, m := range modes {
		modeStrs = append(modeStrs, string(m))
	}
	return strings.Join(modeStrs, ", ")
}
