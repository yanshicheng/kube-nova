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
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type SecretCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 Secret
func NewSecretCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecretCreateLogic {
	return &SecretCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecretCreateLogic) SecretCreate(req *types.SecretRequest) (resp string, err error) {
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

	// 初始化 Secret 客户端
	secretClient := client.Secrets()

	// 解析 YAML 字符串为 Secret 对象
	var secret corev1.Secret
	err = yaml.Unmarshal([]byte(req.SecretYamlStr), &secret)
	if err != nil {
		l.Errorf("解析 Secret YAML 失败: %v", err)
		return "", fmt.Errorf("解析 Secret YAML 失败")
	}

	// 设置命名空间
	if secret.Namespace == "" {
		secret.Namespace = workloadInfo.Data.Namespace
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
		utils.AddAnnotations(&secret.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   secret.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 构建创建详情
	dataKeys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)
	dataKeysStr := strings.Join(dataKeys, ", ")
	if dataKeysStr == "" {
		dataKeysStr = "无"
	}

	// 创建 Secret
	_, createErr := secretClient.Create(&secret)
	if createErr != nil {
		l.Errorf("创建 Secret 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "创建 Secret",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 Secret %s 失败, 类型: %s, 包含 %d 个数据项 (keys: %s), 错误原因: %v", username, secret.Namespace, secret.Name, secret.Type, len(secret.Data), dataKeysStr, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 Secret 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "创建 Secret",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 Secret %s, 类型: %s, 包含 %d 个数据项 (keys: %s)", username, secret.Namespace, secret.Name, secret.Type, len(secret.Data), dataKeysStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功创建 Secret: %s", secret.Name)
	return "创建成功", nil
}
