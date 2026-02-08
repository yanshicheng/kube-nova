package cluster

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

type ClusterRoleBindingCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 ClusterRoleBinding
func NewClusterRoleBindingCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingCreateLogic {
	return &ClusterRoleBindingCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingCreateLogic) ClusterRoleBindingCreate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()

	var crb rbacv1.ClusterRoleBinding
	if err := yaml.Unmarshal([]byte(req.YamlStr), &crb); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 注入注解
	utils.AddAnnotations(&crb.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: crb.Name,
	})

	// 格式化主体列表用于审计
	subjectsList := FormatSubjectsList(crb.Subjects)

	_, err = crbOp.Create(&crb)
	if err != nil {
		l.Errorf("创建 ClusterRoleBinding 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 ClusterRoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 创建 ClusterRoleBinding %s 失败, 绑定角色: %s/%s, 错误: %v", username, crb.Name, crb.RoleRef.Kind, crb.RoleRef.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 ClusterRoleBinding 失败")
	}

	l.Infof("用户: %s, 成功创建 ClusterRoleBinding: %s", username, crb.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功创建 ClusterRoleBinding %s, 绑定角色: %s/%s, 主体数量: %d, 主体详情: %s",
		username, crb.Name, crb.RoleRef.Kind, crb.RoleRef.Name, len(crb.Subjects), subjectsList)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 ClusterRoleBinding",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "创建 ClusterRoleBinding 成功", nil
}
