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

type ClusterRoleBindingUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 ClusterRoleBinding
func NewClusterRoleBindingUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingUpdateLogic {
	return &ClusterRoleBindingUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingUpdateLogic) ClusterRoleBindingUpdate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
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

	// 获取旧的 ClusterRoleBinding 用于对比
	oldCRB, err := crbOp.Get(crb.Name)
	if err != nil {
		l.Errorf("获取原 ClusterRoleBinding 失败: %v", err)
		return "", fmt.Errorf("获取原 ClusterRoleBinding 失败")
	}

	// 注入注解
	utils.AddAnnotations(&crb.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: crb.Name,
	})

	// 对比主体变更
	subjectDiff := CompareSubjects(oldCRB.Subjects, crb.Subjects)
	subjectDiffDetail := BuildSubjectDiffDetail(subjectDiff)

	// 对比标签变更
	labelDiff := CompareStringMaps(oldCRB.Labels, crb.Labels)
	labelDiffDetail := ""
	if HasMapChanges(labelDiff) {
		labelDiffDetail = "标签变更: " + BuildMapDiffDetail(labelDiff, true)
	}

	// 检查 RoleRef 是否变更（注意：K8s 不允许修改 RoleRef，但记录尝试）
	roleRefChanged := oldCRB.RoleRef.Name != crb.RoleRef.Name || oldCRB.RoleRef.Kind != crb.RoleRef.Kind
	var roleRefInfo string
	if roleRefChanged {
		roleRefInfo = fmt.Sprintf("角色引用变更: %s/%s → %s/%s (注意: K8s 不支持修改 RoleRef)",
			oldCRB.RoleRef.Kind, oldCRB.RoleRef.Name, crb.RoleRef.Kind, crb.RoleRef.Name)
	}

	_, err = crbOp.Update(&crb)
	if err != nil {
		l.Errorf("更新 ClusterRoleBinding 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 ClusterRoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 更新 ClusterRoleBinding %s 失败, 错误: %v", username, crb.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("更新 ClusterRoleBinding 失败")
	}

	l.Infof("用户: %s, 成功更新 ClusterRoleBinding: %s", username, crb.Name)

	// 构建详细审计信息
	var changeParts []string
	changeParts = append(changeParts, fmt.Sprintf("用户 %s 成功更新 ClusterRoleBinding %s", username, crb.Name))
	changeParts = append(changeParts, fmt.Sprintf("绑定角色: %s/%s", crb.RoleRef.Kind, crb.RoleRef.Name))
	changeParts = append(changeParts, fmt.Sprintf("主体数量: %d → %d", len(oldCRB.Subjects), len(crb.Subjects)))

	if HasSubjectChanges(subjectDiff) {
		changeParts = append(changeParts, subjectDiffDetail)
	}
	if labelDiffDetail != "" {
		changeParts = append(changeParts, labelDiffDetail)
	}
	if roleRefInfo != "" {
		changeParts = append(changeParts, roleRefInfo)
	}

	auditDetail := joinNonEmpty(", ", changeParts...)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 ClusterRoleBinding",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "更新 ClusterRoleBinding 成功", nil
}
