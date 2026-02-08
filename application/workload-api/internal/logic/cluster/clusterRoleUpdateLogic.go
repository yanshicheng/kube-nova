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

type ClusterRoleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 ClusterRole
func NewClusterRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleUpdateLogic {
	return &ClusterRoleUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleUpdateLogic) ClusterRoleUpdate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
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
	_, err = crOp.Update(&cr)
	if err != nil {
		l.Errorf("更新 ClusterRole 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 ClusterRole",
			ActionDetail: fmt.Sprintf("更新 ClusterRole %s 失败，错误: %v", cr.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("更新 ClusterRole 失败")
	}

	l.Infof("用户: %s, 成功更新 ClusterRole: %s", username, cr.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 ClusterRole",
		ActionDetail: fmt.Sprintf("更新 ClusterRole %s 成功，规则数量: %d", cr.Name, len(cr.Rules)),
		Status:       1,
	})
	return "更新 ClusterRole 成功", nil
}
