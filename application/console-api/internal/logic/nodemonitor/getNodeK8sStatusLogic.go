package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeK8sStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点 K8s 状态
func NewGetNodeK8sStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeK8sStatusLogic {
	return &GetNodeK8sStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeK8sStatusLogic) GetNodeK8sStatus(req *types.GetNodeK8sStatusRequest) (resp *types.GetNodeK8sStatusResponse, err error) {

	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	status, err := node.GetNodeK8sStatus(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点 K8s 状态失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeK8sStatusResponse{
		Data: convertNodeK8sStatus(status),
	}

	l.Infof("获取节点 K8s 状态成功: Node=%s", req.NodeName)
	return resp, nil
}
