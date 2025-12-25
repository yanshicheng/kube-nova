package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 配置指标
func NewGetNamespaceConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceConfigLogic {
	return &GetNamespaceConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceConfigLogic) GetNamespaceConfig(req *types.GetNamespaceConfigRequest) (resp *types.GetNamespaceConfigResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()

	metrics, err := namespace.GetNamespaceConfig(req.Namespace)
	if err != nil {
		l.Errorf("获取 Namespace Config 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceConfigResponse{
		Data: convertNamespaceConfigMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Config 成功", req.Namespace)
	return resp, nil
}
