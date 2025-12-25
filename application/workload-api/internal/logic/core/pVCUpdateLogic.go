// pVCUpdateLogic.go
package core

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

type PVCUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPVCUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCUpdateLogic {
	return &PVCUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCUpdateLogic) PVCUpdate(req *types.ClusterNamespaceResourceUpdateRequest) (resp string, err error) {
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
	// 获取项目详情

	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&pvc.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   pvc.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	// 更新 PVC
	updateErr := pvcOp.Update(req.Namespace, req.Name, &pvc)
	if updateErr != nil {
		l.Errorf("更新 PVC 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 PVC",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 PVC %s 失败, 错误原因: %v", username, req.Namespace, req.Name, updateErr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 PVC 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 PVC",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 PVC %s", username, req.Namespace, req.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功更新 PVC: %s/%s", username, req.Namespace, pvc.Name)
	return "更新 PVC 成功", nil
}
