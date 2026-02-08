package core

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 ConfigMap
func NewConfigMapDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapDeleteLogic {
	return &ConfigMapDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapDeleteLogic) ConfigMapDelete(req *types.DefaultNameRequest) (resp string, err error) {
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

	// 获取 ConfigMap 详情用于审计
	existingCM, _ := configMapClient.Get(workloadInfo.Data.Namespace, req.Name)
	var dataKeysStr string
	var dataCount int
	if existingCM != nil {
		dataCount = len(existingCM.Data)
		dataKeys := make([]string, 0, len(existingCM.Data))
		for k := range existingCM.Data {
			dataKeys = append(dataKeys, k)
		}
		sort.Strings(dataKeys)
		dataKeysStr = strings.Join(dataKeys, ", ")
	}

	// 删除 ConfigMap
	deleteErr := configMapClient.Delete(workloadInfo.Data.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 ConfigMap 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "删除 ConfigMap",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 ConfigMap %s 失败, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 ConfigMap 失败")
	}

	// 记录成功的审计日志
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 ConfigMap %s", username, workloadInfo.Data.Namespace, req.Name)
	if dataCount > 0 {
		auditDetail = fmt.Sprintf("%s, 删除前包含 %d 个数据项 (keys: %s)", auditDetail, dataCount, dataKeysStr)
	}

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "删除 ConfigMap",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功删除 ConfigMap: %s", req.Name)
	return "删除成功", nil
}
