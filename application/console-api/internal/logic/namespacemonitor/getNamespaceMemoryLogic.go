package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceMemoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 内存指标
func NewGetNamespaceMemoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceMemoryLogic {
	return &GetNamespaceMemoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceMemoryLogic) GetNamespaceMemory(req *types.GetNamespaceMemoryRequest) (resp *types.GetNamespaceMemoryResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := namespace.GetNamespaceMemory(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Memory 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceMemoryResponse{
		Data: convertNamespaceMemoryMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Memory 指标成功", req.Namespace)
	return resp, nil
}
