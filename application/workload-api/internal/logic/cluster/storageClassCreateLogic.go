package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 StorageClass
func NewStorageClassCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassCreateLogic {
	return &StorageClassCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassCreateLogic) StorageClassCreate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	scOp := client.StorageClasses()

	var sc storagev1.StorageClass
	if err := yaml.Unmarshal([]byte(req.YamlStr), &sc); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}
	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   sc.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&sc.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   sc.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	_, err = scOp.Create(&sc)
	if err != nil {
		l.Errorf("创建 StorageClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 StorageClass",
			ActionDetail: fmt.Sprintf("创建 StorageClass %s 失败，Provisioner: %s，错误: %v", sc.Name, sc.Provisioner, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 StorageClass 失败")
	}

	l.Infof("用户: %s, 成功创建 StorageClass: %s", username, sc.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 StorageClass",
		ActionDetail: fmt.Sprintf("创建 StorageClass %s 成功，Provisioner: %s", sc.Name, sc.Provisioner),
		Status:       1,
	})
	return "创建 StorageClass 成功", nil
}
