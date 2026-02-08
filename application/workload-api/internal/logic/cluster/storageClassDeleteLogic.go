package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 StorageClass
func NewStorageClassDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassDeleteLogic {
	return &StorageClassDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassDeleteLogic) StorageClassDelete(req *types.ClusterResourceDeleteRequest) (resp string, err error) {
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

	// 获取 StorageClass 详情用于审计
	existingSC, _ := scOp.Get(req.Name)
	var provisioner string
	var isDefault bool
	if existingSC != nil {
		provisioner = existingSC.Provisioner
		if existingSC.Annotations != nil {
			if val, ok := existingSC.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				isDefault = true
			}
		}
	}

	// 获取关联的 PV 信息
	pvResponse, _ := scOp.GetAssociatedPVs(req.Name)
	var pvInfo string
	if pvResponse != nil && pvResponse.PVCount > 0 {
		pvInfo = fmt.Sprintf(", 关联 PV: %d 个 (已绑定: %d, 可用: %d)",
			pvResponse.PVCount, pvResponse.BoundPVCount, pvResponse.AvailablePVCount)
	}

	err = scOp.Delete(req.Name)
	if err != nil {
		l.Errorf("删除 StorageClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 StorageClass",
			ActionDetail: fmt.Sprintf("用户 %s 删除 StorageClass %s 失败, 错误: %v", username, req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("删除 StorageClass 失败")
	}

	l.Infof("用户: %s, 成功删除 StorageClass: %s", username, req.Name)

	// 构建详细审计信息
	var defaultInfo string
	if isDefault {
		defaultInfo = ", 默认: 是"
	}

	auditDetail := fmt.Sprintf("用户 %s 成功删除 StorageClass %s, Provisioner: %s%s%s",
		username, req.Name, provisioner, defaultInfo, pvInfo)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 StorageClass",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "删除 StorageClass 成功", nil
}
