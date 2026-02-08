package selector

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceLabelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 标签
func NewGetNamespaceLabelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceLabelsLogic {
	return &GetNamespaceLabelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceLabelsLogic) GetNamespaceLabels(req *types.NamespaceLabelsRequest) (resp *types.NamespaceLabelsResponse, err error) {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 获取 Namespace 操作器
	namespaceOp := client.Namespaces()

	// 获取 Namespace 对象
	ns, err := namespaceOp.Get(req.Namespace)
	if err != nil {
		l.Errorf("获取 Namespace 失败: %v", err)
		return nil, fmt.Errorf("获取 Namespace 失败: %v", err)
	}

	labels := ns.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	return &types.NamespaceLabelsResponse{
		Labels: labels,
	}, nil
}
