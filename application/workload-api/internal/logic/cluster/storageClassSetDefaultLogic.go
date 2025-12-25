package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassSetDefaultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 设置默认 StorageClass
func NewStorageClassSetDefaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassSetDefaultLogic {
	return &StorageClassSetDefaultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassSetDefaultLogic) StorageClassSetDefault(req *types.SetDefaultStorageClassRequest) (resp string, err error) {
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
	err = scOp.SetDefault(req.Name)
	if err != nil {
		l.Errorf("设置默认 StorageClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "设置默认 StorageClass",
			ActionDetail: fmt.Sprintf("设置默认 StorageClass %s 失败，错误: %v", req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("设置默认 StorageClass 失败")
	}

	l.Infof("用户: %s, 成功设置默认 StorageClass: %s", username, req.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "设置默认 StorageClass",
		ActionDetail: fmt.Sprintf("设置默认 StorageClass %s 成功", req.Name),
		Status:       1,
	})
	return "设置默认 StorageClass 成功", nil
}
