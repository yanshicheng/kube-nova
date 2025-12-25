package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeSystemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点系统指标
func NewGetNodeSystemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeSystemLogic {
	return &GetNodeSystemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeSystemLogic) GetNodeSystem(req *types.GetNodeSystemRequest) (resp *types.GetNodeSystemResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	system, err := node.GetNodeSystem(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点系统指标失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeSystemResponse{
		Data: convertNodeSystemMetrics(system),
	}

	l.Infof("获取节点系统指标成功: Node=%s", req.NodeName)
	return resp, nil
}
