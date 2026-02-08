package core

import (
	"context"
	"fmt"
	"strings"

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

	// 对比 Data 变更
	dataDiff := CompareStringMaps(existingCM.Data, req.Data)
	dataChangeDetail := BuildMapDiffDetail(dataDiff, AuditDetailConfig.RecordConfigMapValues)

	// 对比 Labels 变更
	labelsDiff := CompareStringMaps(existingCM.Labels, req.Labels)
	labelsChangeDetail := BuildMapDiffDetail(labelsDiff, false)

	// 对比 Annotations 变更
	annotationsDiff := CompareStringMaps(existingCM.Annotations, req.Annotations)
	annotationsChangeDetail := BuildMapDiffDetail(annotationsDiff, false)

	// 构建变更详情
	var changeDetails []string
	if HasMapChanges(dataDiff) {
		changeDetails = append(changeDetails, fmt.Sprintf("Data变更: %s", dataChangeDetail))
	}
	if HasMapChanges(labelsDiff) {
		changeDetails = append(changeDetails, fmt.Sprintf("Labels变更: %s", labelsChangeDetail))
	}
	if HasMapChanges(annotationsDiff) {
		changeDetails = append(changeDetails, fmt.Sprintf("Annotations变更: %s", annotationsChangeDetail))
	}

	changeDetailStr := "无变更"
	if len(changeDetails) > 0 {
		changeDetailStr = strings.Join(changeDetails, "; ")
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
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 ConfigMap %s 失败, 错误原因: %v, 变更内容: %s", username, workloadInfo.Data.Namespace, req.Name, updateErr, changeDetailStr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 ConfigMap 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "更新 ConfigMap",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 ConfigMap %s, %s", username, workloadInfo.Data.Namespace, req.Name, changeDetailStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功更新 ConfigMap: %s", req.Name)
	return "更新成功", nil
}
