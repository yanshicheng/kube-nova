package networkpolicy

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"sigs.k8s.io/yaml"
)

type NetworkPolicyCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 NetworkPolicy
func NewNetworkPolicyCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NetworkPolicyCreateLogic {
	return &NetworkPolicyCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NetworkPolicyCreateLogic) NetworkPolicyCreate(req *types.NetworkPolicyYamlRequest) (resp string, err error) {
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

	// 解析 YAML 为 NetworkPolicy 对象
	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal([]byte(req.YamlStr), &np); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 确保命名空间正确
	np.Namespace = req.Namespace

	// 获取 NetworkPolicy 操作器
	networkPolicyOp := client.NetworkPolicies()

	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	}

	// 注入注解
	utils.AddAnnotations(&np.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   np.Name,
		ProjectName:   projectDetail.ProjectNameCn,
		WorkspaceName: projectDetail.WorkspaceNameCn,
		ProjectUuid:   projectDetail.ProjectUuid,
	})

	// 检查是否已存在
	_, getErr := networkPolicyOp.Get(req.Namespace, np.Name)
	if getErr == nil {
		l.Errorf("NetworkPolicy %s/%s 已存在", req.Namespace, np.Name)
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid: req.ClusterUuid,
			Title:       "创建 NetworkPolicy",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 NetworkPolicy %s 失败, 资源已存在",
				username, req.Namespace, np.Name),
			Status: 0,
		})
		return "", fmt.Errorf("NetworkPolicy %s/%s 已存在", req.Namespace, np.Name)
	}

	// 调用 Create 方法
	_, createErr := networkPolicyOp.Create(&np)
	if createErr != nil {
		l.Errorf("创建 NetworkPolicy 失败: %v", createErr)
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid: req.ClusterUuid,
			Title:       "创建 NetworkPolicy",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 NetworkPolicy %s 失败, 错误原因: %v",
				username, req.Namespace, np.Name, createErr),
			Status: 0,
		})
		return "", fmt.Errorf("创建 NetworkPolicy 失败: %v", createErr)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid: req.ClusterUuid,
		Title:       "创建 NetworkPolicy",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 NetworkPolicy %s",
			username, req.Namespace, np.Name),
		Status: 1,
	})

	l.Infof("用户: %s, 成功创建 NetworkPolicy %s/%s", username, req.Namespace, np.Name)

	return fmt.Sprintf("NetworkPolicy %s/%s 创建成功", req.Namespace, np.Name), nil
}
