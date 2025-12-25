package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceTopPodsByNetworkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Top Pods (Network)
func NewGetNamespaceTopPodsByNetworkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceTopPodsByNetworkLogic {
	return &GetNamespaceTopPodsByNetworkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceTopPodsByNetworkLogic) GetNamespaceTopPodsByNetwork(req *types.GetNamespaceTopPodsByNetworkRequest) (resp *types.GetNamespaceTopPodsByNetworkResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	rankings, err := namespace.GetTopPodsByNetwork(req.Namespace, req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Top Pods by Network 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceTopPodsByNetworkResponse{
		Data: convertResourceRankingList(rankings),
	}

	l.Infof("获取 Namespace %s Top %d Pods by Network 成功", req.Namespace, req.Limit)
	return resp, nil
}
