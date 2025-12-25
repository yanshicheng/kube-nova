package monitoring

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"sigs.k8s.io/yaml"
)

type PrometheusRuleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 PrometheusRule
func NewPrometheusRuleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrometheusRuleUpdateLogic {
	return &PrometheusRuleUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PrometheusRuleUpdateLogic) PrometheusRuleUpdate(req *types.MonitoringResourceYamlRequest) (resp string, err error) {

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 解析 YAML 为 PrometheusRule 对象
	var rule monitoringv1.PrometheusRule
	if err := yaml.Unmarshal([]byte(req.YamlStr), &rule); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 确保命名空间正确
	rule.Namespace = req.Namespace

	// 获取 PrometheusRule 操作器
	ruleOp := client.PrometheusRule()
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&rule.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   rule.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 调用 Update 方法
	updateErr := ruleOp.Update(req.Namespace, rule.Name, &rule)
	if updateErr != nil {
		l.Errorf("创建  失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 prometheus rule",
			ActionDetail: fmt.Sprintf(" 在命名空间 %s 创建 prometheus rule %s 失败, 错误原因: %v", req.Namespace, rule.Name, updateErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 ConfigMap 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 prometheus rule",
		ActionDetail: fmt.Sprintf(" 在命名空间 %s 成功创建 prometheus rule %s", req.Namespace, rule.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	return fmt.Sprintf("PrometheusRule %s/%s 更新成功", rule.Namespace, rule.Name), nil
}
