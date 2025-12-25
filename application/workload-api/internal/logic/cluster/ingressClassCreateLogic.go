package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 IngressClass
func NewIngressClassCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassCreateLogic {
	return &IngressClassCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassCreateLogic) IngressClassCreate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()

	var ic networkingv1.IngressClass
	if err := yaml.Unmarshal([]byte(req.YamlStr), &ic); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}
	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   ic.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&ic.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   ic.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	_, err = icOp.Create(&ic)
	if err != nil {
		l.Errorf("创建 IngressClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 IngressClass",
			ActionDetail: fmt.Sprintf("创建 IngressClass %s 失败，控制器: %s，错误: %v", ic.Name, ic.Spec.Controller, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 IngressClass 失败")
	}

	l.Infof("用户: %s, 成功创建 IngressClass: %s", username, ic.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 IngressClass",
		ActionDetail: fmt.Sprintf("创建 IngressClass %s 成功，控制器: %s", ic.Name, ic.Spec.Controller),
		Status:       1,
	})
	return "创建 IngressClass 成功", nil
}
