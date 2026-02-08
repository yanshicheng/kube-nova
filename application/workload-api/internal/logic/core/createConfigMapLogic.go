package core

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateConfigMapLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 ConfigMap
func NewCreateConfigMapLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateConfigMapLogic {
	return &CreateConfigMapLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateConfigMapLogic) CreateConfigMap(req *types.ConfigMapRequest) (resp string, err error) {
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

	// 构造 ConfigMap 对象
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   workloadInfo.Data.Namespace,
			Labels:      req.Labels,
			Annotations: req.Annotations,
		},
		Data: req.Data,
	}

	// 获取项目详情并注入注解
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: workloadInfo.Data.ClusterUuid,
		Namespace:   workloadInfo.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		utils.AddAnnotations(&configMap.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   configMap.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 构建创建详情
	dataKeys := make([]string, 0, len(req.Data))
	for k := range req.Data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)
	dataKeysStr := strings.Join(dataKeys, ", ")
	if dataKeysStr == "" {
		dataKeysStr = "无"
	}

	// 创建 ConfigMap
	_, createErr := configMapClient.Create(configMap)
	if createErr != nil {
		l.Errorf("创建 ConfigMap 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "创建 ConfigMap",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 ConfigMap %s 失败, 包含 %d 个数据项 (keys: %s), 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, len(req.Data), dataKeysStr, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 ConfigMap 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "创建 ConfigMap",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 ConfigMap %s, 包含 %d 个数据项 (keys: %s)", username, workloadInfo.Data.Namespace, req.Name, len(req.Data), dataKeysStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功创建 ConfigMap: %s", req.Name)
	return "创建成功", nil
}
