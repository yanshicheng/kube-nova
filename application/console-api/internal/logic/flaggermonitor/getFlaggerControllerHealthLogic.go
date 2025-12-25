package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetFlaggerControllerHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Flagger Controller 健康状态
func NewGetFlaggerControllerHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFlaggerControllerHealthLogic {
	return &GetFlaggerControllerHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFlaggerControllerHealthLogic) GetFlaggerControllerHealth(req *types.GetFlaggerControllerHealthRequest) (resp *types.GetFlaggerControllerHealthResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	health, err := flagger.GetFlaggerControllerHealth(timeRange)
	if err != nil {
		l.Errorf("获取 Flagger Controller 健康状态失败: %v", err)
		return nil, err
	}

	resp = &types.GetFlaggerControllerHealthResponse{
		Data: convertFlaggerControllerHealth(health),
	}

	l.Infof("获取 Flagger Controller 健康状态成功")
	return resp, nil
}
