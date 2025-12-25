package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingUpdateLogic {
	return &RoleBindingUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingUpdateLogic) RoleBindingUpdate(req *types.ClusterNamespaceResourceUpdateRequest) (resp string, err error) {
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
	// 更新 RoleBinding
	updateErr := rbOp.Update(req.Namespace, req.Name, &rb)
	if updateErr != nil {
		l.Errorf("更新 RoleBinding 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 RoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 RoleBinding %s 失败, 错误原因: %v", username, req.Namespace, req.Name, updateErr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 RoleBinding 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 RoleBinding",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 RoleBinding %s, 绑定角色: %s/%s", username, req.Namespace, req.Name, rb.RoleRef.Kind, rb.RoleRef.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功更新 RoleBinding: %s/%s", username, req.Namespace, rb.Name)
	return "更新 RoleBinding 成功", nil
}
