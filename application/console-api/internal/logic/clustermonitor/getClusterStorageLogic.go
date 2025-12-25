package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterStorageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群存储统计
func NewGetClusterStorageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterStorageLogic {
	return &GetClusterStorageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterStorageLogic) GetClusterStorage(req *types.GetClusterStorageRequest) (resp *types.GetClusterStorageResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()

	storage, err := cluster.GetClusterStorage()
	if err != nil {
		l.Errorf("获取集群存储统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterStorageResponse{
		Data: convertClusterStorageMetrics(storage),
	}

	l.Infof("获取集群存储统计成功")
	return resp, nil
}
