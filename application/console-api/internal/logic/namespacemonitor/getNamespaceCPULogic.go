package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceCPULogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace CPU 指标
func NewGetNamespaceCPULogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceCPULogic {
	return &GetNamespaceCPULogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceCPULogic) GetNamespaceCPU(req *types.GetNamespaceCPURequest) (resp *types.GetNamespaceCPUResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := namespace.GetNamespaceCPU(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace CPU 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceCPUResponse{
		Data: convertNamespaceCPUMetrics(metrics),
	}

	l.Infof("获取 Namespace %s CPU 指标成功", req.Namespace)
	return resp, nil
}
