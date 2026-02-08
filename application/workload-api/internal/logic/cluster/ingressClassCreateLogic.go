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

	// 注入注解
	utils.AddAnnotations(&ic.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: ic.Name,
	})

	// 检查是否设置为默认
	isDefault := false
	if ic.Annotations != nil {
		if val, ok := ic.Annotations["ingressclass.kubernetes.io/is-default-class"]; ok && val == "true" {
			isDefault = true
		}
	}

	// 检查是否有参数配置
	hasParams := ic.Spec.Parameters != nil

	configDetail := FormatIngressClassConfig(ic.Spec.Controller, isDefault, hasParams)

	// 如果有参数，添加参数详情
	var paramsDetail string
	if hasParams {
		paramsDetail = fmt.Sprintf(", 参数: Kind=%s, Name=%s", ic.Spec.Parameters.Kind, ic.Spec.Parameters.Name)
	}

	_, err = icOp.Create(&ic)
	if err != nil {
		l.Errorf("创建 IngressClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 IngressClass",
			ActionDetail: fmt.Sprintf("用户 %s 创建 IngressClass %s 失败, 控制器: %s, 错误: %v", username, ic.Name, ic.Spec.Controller, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 IngressClass 失败")
	}

	l.Infof("用户: %s, 成功创建 IngressClass: %s", username, ic.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功创建 IngressClass %s, %s%s", username, ic.Name, configDetail, paramsDetail)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 IngressClass",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "创建 IngressClass 成功", nil
}
