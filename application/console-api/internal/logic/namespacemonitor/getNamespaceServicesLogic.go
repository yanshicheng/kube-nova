package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Service 统计
func NewGetNamespaceServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceServicesLogic {
	return &GetNamespaceServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceServicesLogic) GetNamespaceServices(req *types.GetNamespaceServicesRequest) (resp *types.GetNamespaceServicesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()

	metrics, err := namespace.GetNamespaceServices(req.Namespace)
	if err != nil {
		l.Errorf("获取 Namespace Services 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceServicesResponse{
		Data: convertNamespaceServiceStatistics(metrics),
	}

	l.Infof("获取 Namespace %s Services 成功", req.Namespace)
	return resp, nil
}
