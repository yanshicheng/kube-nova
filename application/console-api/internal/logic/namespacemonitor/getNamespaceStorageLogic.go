package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceStorageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 存储指标
func NewGetNamespaceStorageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceStorageLogic {
	return &GetNamespaceStorageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceStorageLogic) GetNamespaceStorage(req *types.GetNamespaceStorageRequest) (resp *types.GetNamespaceStorageResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()

	metrics, err := namespace.GetNamespaceStorage(req.Namespace)
	if err != nil {
		l.Errorf("获取 Namespace Storage 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceStorageResponse{
		Data: convertNamespaceStorageMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Storage 成功", req.Namespace)
	return resp, nil
}
