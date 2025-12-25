package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVCanDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查 PV 是否可删除
func NewPVCanDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCanDeleteLogic {
	return &PVCanDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCanDeleteLogic) PVCanDelete(req *types.ClusterResourceNameRequest) (resp *types.CanDeleteResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()
	canDelete, warning, err := pvOp.CanDelete(req.Name)
	if err != nil {
		l.Errorf("检查 PV 是否可删除失败: %v", err)
		return nil, fmt.Errorf("检查 PV 是否可删除失败")
	}

	return &types.CanDeleteResponse{
		CanDelete: canDelete,
		Warning:   warning,
	}, nil
}
