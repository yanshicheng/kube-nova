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

type ClusterRoleCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 ClusterRole
func NewClusterRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleCreateLogic {
	return &ClusterRoleCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleCreateLogic) ClusterRoleCreate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()

	var cr rbacv1.ClusterRole
	if err := yaml.Unmarshal([]byte(req.YamlStr), &cr); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 注入注解
	utils.AddAnnotations(&cr.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: cr.Name,
	})

	// 构建规则摘要用于审计
	rulesSummary := FormatPolicyRulesShort(cr.Rules)

	// 检查是否有聚合规则
	var aggregationInfo string
	if cr.AggregationRule != nil && len(cr.AggregationRule.ClusterRoleSelectors) > 0 {
		aggregationInfo = fmt.Sprintf(", 聚合规则: %d 个选择器", len(cr.AggregationRule.ClusterRoleSelectors))
	}

	_, err = crOp.Create(&cr)
	if err != nil {
		l.Errorf("创建 ClusterRole 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 ClusterRole",
			ActionDetail: fmt.Sprintf("用户 %s 创建 ClusterRole %s 失败, 规则数量: %d, 错误: %v", username, cr.Name, len(cr.Rules), err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 ClusterRole 失败")
	}

	l.Infof("用户: %s, 成功创建 ClusterRole: %s", username, cr.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功创建 ClusterRole %s, 规则数量: %d, 规则摘要: %s%s",
		username, cr.Name, len(cr.Rules), rulesSummary, aggregationInfo)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 ClusterRole",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "创建 ClusterRole 成功", nil
}
