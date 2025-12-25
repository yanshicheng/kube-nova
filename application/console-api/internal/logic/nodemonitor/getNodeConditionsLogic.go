package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeConditionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点状况
func NewGetNodeConditionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeConditionsLogic {
	return &GetNodeConditionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeConditionsLogic) GetNodeConditions(req *types.GetNodeConditionsRequest) (resp *types.GetNodeConditionsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()

	conditions, err := node.GetNodeConditions(req.NodeName)
	if err != nil {
		l.Errorf("获取节点状况失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeConditionsResponse{}
	for _, cond := range conditions {
		resp.Data = append(resp.Data, types.NodeCondition{
			Type:               cond.Type,
			Status:             cond.Status,
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: cond.LastTransitionTime.Unix(),
		})
	}

	l.Infof("获取节点状况成功: Node=%s, Count=%d", req.NodeName, len(conditions))
	return resp, nil
}
