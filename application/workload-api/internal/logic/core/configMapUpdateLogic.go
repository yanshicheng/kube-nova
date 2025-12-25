package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 ConfigMap
func NewConfigMapUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapUpdateLogic {
	return &ConfigMapUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapUpdateLogic) ConfigMapUpdate(req *types.ConfigMapRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 ConfigMap 客户端
	configMapClient := client.ConfigMaps()

	// 获取现有的 ConfigMap
	existingCM, err := configMapClient.Get(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 ConfigMap 失败: %v", err)
		return "", fmt.Errorf("获取现有 ConfigMap 失败")
	}

	// 更新字段
	existingCM.Data = req.Data
	if req.Labels != nil {
		existingCM.Labels = req.Labels
	}
	if req.Annotations != nil {
		existingCM.Annotations = req.Annotations
	}
	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: workloadInfo.Data.ClusterUuid,
		Namespace:   workloadInfo.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&existingCM.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   existingCM.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	// 执行更新
	_, updateErr := configMapClient.Update(existingCM)
	if updateErr != nil {
		l.Errorf("更新 ConfigMap 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "更新 ConfigMap",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 ConfigMap %s 失败, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, updateErr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 ConfigMap 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "更新 ConfigMap",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 ConfigMap %s, 更新后包含 %d 个数据项", username, workloadInfo.Data.Namespace, req.Name, len(req.Data)),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功更新 ConfigMap: %s", req.Name)
	return "更新成功", nil
}
