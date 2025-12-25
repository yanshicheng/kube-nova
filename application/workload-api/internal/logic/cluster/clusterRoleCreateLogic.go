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
	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   cr.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&cr.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   cr.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	_, err = crOp.Create(&cr)
	if err != nil {
		l.Errorf("创建 ClusterRole 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 ClusterRole",
			ActionDetail: fmt.Sprintf("创建 ClusterRole %s 失败，错误: %v", cr.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 ClusterRole 失败")
	}

	l.Infof("用户: %s, 成功创建 ClusterRole: %s", username, cr.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 ClusterRole",
		ActionDetail: fmt.Sprintf("创建 ClusterRole %s 成功，规则数量: %d", cr.Name, len(cr.Rules)),
		Status:       1,
	})
	return "创建 ClusterRole 成功", nil
}
