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

type PVCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 PV
func NewPVCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCreateLogic {
	return &PVCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCreateLogic) PVCreate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
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

	capacity := ""
	if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = qty.String()
	}
	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   pv.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&pv.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   pv.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	_, err = pvOp.Create(&pv)
	if err != nil {
		l.Errorf("创建 PV 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 PersistentVolume",
			ActionDetail: fmt.Sprintf("创建 PersistentVolume %s 失败，容量: %s，StorageClass: %s，错误: %v", pv.Name, capacity, pv.Spec.StorageClassName, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 PV 失败")
	}

	l.Infof("用户: %s, 成功创建 PV: %s", username, pv.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 PersistentVolume",
		ActionDetail: fmt.Sprintf("创建 PersistentVolume %s 成功，容量: %s，StorageClass: %s", pv.Name, capacity, pv.Spec.StorageClassName),
		Status:       1,
	})
	return "创建 PV 成功", nil
}
