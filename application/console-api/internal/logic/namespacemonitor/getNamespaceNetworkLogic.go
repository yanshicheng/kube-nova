package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceNetworkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 网络指标
func NewGetNamespaceNetworkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceNetworkLogic {
	return &GetNamespaceNetworkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceNetworkLogic) GetNamespaceNetwork(req *types.GetNamespaceNetworkRequest) (resp *types.GetNamespaceNetworkResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := namespace.GetNamespaceNetwork(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Network 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceNetworkResponse{
		Data: convertNamespaceNetworkMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Network 指标成功", req.Namespace)
	return resp, nil
}
