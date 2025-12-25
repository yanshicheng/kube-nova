package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassCanDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查 IngressClass 是否可删除
func NewIngressClassCanDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassCanDeleteLogic {
	return &IngressClassCanDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassCanDeleteLogic) IngressClassCanDelete(req *types.IngressClassNameRequest) (resp *types.CanDeleteResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	canDelete, warning, err := icOp.CanDelete(req.Name)
	if err != nil {
		l.Errorf("检查 IngressClass 是否可删除失败: %v", err)
		return nil, fmt.Errorf("检查 IngressClass 是否可删除失败")
	}

	return &types.CanDeleteResponse{
		CanDelete: canDelete,
		Warning:   warning,
	}, nil
}
