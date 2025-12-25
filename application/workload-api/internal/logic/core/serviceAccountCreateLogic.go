// serviceAccountCreateLogic.go
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

type ServiceAccountCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceAccountCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountCreateLogic {
	return &ServiceAccountCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountCreateLogic) ServiceAccountCreate(req *types.DefaultCoreCreateRequest) (resp string, err error) {
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

	// 获取 ServiceAccount operator
	saOp := client.ServiceAccounts()

	// 解析 YAML
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal([]byte(req.YamlStr), &sa); err != nil {
		l.Errorf("解析 ServiceAccount YAML 失败: %v", err)
		return "", fmt.Errorf("解析 ServiceAccount YAML 失败")
	}

	// 确保命名空间正确
	if sa.Namespace == "" {
		sa.Namespace = req.Namespace
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
		utils.AddAnnotations(&sa.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   sa.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	// 创建 ServiceAccount
	createErr := saOp.Create(&sa)
	if createErr != nil {
		l.Errorf("创建 ServiceAccount 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 ServiceAccount",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 ServiceAccount %s 失败, 错误原因: %v", username, sa.Namespace, sa.Name, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 ServiceAccount 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 ServiceAccount",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 ServiceAccount %s", username, sa.Namespace, sa.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功创建 ServiceAccount: %s/%s", username, sa.Namespace, sa.Name)
	return "创建 ServiceAccount 成功", nil
}
