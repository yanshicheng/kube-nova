package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingCreateLogic {
	return &RoleBindingCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingCreateLogic) RoleBindingCreate(req *types.DefaultCoreCreateRequest) (resp string, err error) {
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

	// 获取 RoleBinding operator
	rbOp := client.RoleBindings()

	// 解析 YAML
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal([]byte(req.YamlStr), &rb); err != nil {
		l.Errorf("解析 RoleBinding YAML 失败: %v", err)
		return "", fmt.Errorf("解析 RoleBinding YAML 失败")
	}

	// 确保命名空间正确
	if rb.Namespace == "" {
		rb.Namespace = req.Namespace
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
		utils.AddAnnotations(&rb.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   rb.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 构建主体信息用于审计
	subjectsSummary := l.buildSubjectsSummary(rb.Subjects)

	// 创建 RoleBinding
	createErr := rbOp.Create(&rb)
	if createErr != nil {
		l.Errorf("创建 RoleBinding 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 RoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 RoleBinding %s 失败, 绑定角色: %s/%s, 主体: %s, 错误原因: %v", username, rb.Namespace, rb.Name, rb.RoleRef.Kind, rb.RoleRef.Name, subjectsSummary, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 RoleBinding 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 RoleBinding",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 RoleBinding %s, 绑定角色: %s/%s, 主体: %s (%d个)", username, rb.Namespace, rb.Name, rb.RoleRef.Kind, rb.RoleRef.Name, subjectsSummary, len(rb.Subjects)),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功创建 RoleBinding: %s/%s", username, rb.Namespace, rb.Name)
	return "创建 RoleBinding 成功", nil
}

// buildSubjectsSummary 构建主体摘要用于审计日志
func (l *RoleBindingCreateLogic) buildSubjectsSummary(subjects []rbacv1.Subject) string {
	if len(subjects) == 0 {
		return "无"
	}

	var summaries []string
	for _, s := range subjects {
		if s.Namespace != "" {
			summaries = append(summaries, fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name))
		} else {
			summaries = append(summaries, fmt.Sprintf("%s/%s", s.Kind, s.Name))
		}
	}

	result := strings.Join(summaries, ", ")
	if len(result) > 150 {
		return result[:150] + "..."
	}
	return result
}
