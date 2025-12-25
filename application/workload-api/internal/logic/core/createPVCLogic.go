package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
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

	// 创建 PVC
	createErr := pvcOp.Create(&pvc)
	if createErr != nil {
		l.Errorf("创建 PVC 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 PVC",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 PVC %s 失败, 存储类: %s, 请求大小: %s, 错误原因: %v", username, pvc.Namespace, pvc.Name, storageClass, requestStorage, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 PVC 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 PVC",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 PVC %s, 存储类: %s, 请求大小: %s", username, pvc.Namespace, pvc.Name, storageClass, requestStorage),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功创建 PVC: %s/%s", username, pvc.Namespace, pvc.Name)
	return "创建 PVC 成功", nil
}
