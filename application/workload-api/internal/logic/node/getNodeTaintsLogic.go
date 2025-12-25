package node

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeTaintsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeTaintsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeTaintsLogic {
	return &GetNodeTaintsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeTaintsLogic) GetNodeTaints(req *types.DefaultNodeNameRequest) (resp []types.NodeTaint, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取节点污点
	nodeOperator := client.Node()
	taints, err := nodeOperator.GetTaints(req.NodeName)
	if err != nil {
		l.Errorf("获取节点污点失败: %v", err)
		return nil, fmt.Errorf("获取节点污点失败")
	}

	// 4. 转换为响应格式，并标记系统污点不可删除
	resp = make([]types.NodeTaint, 0, len(taints))
	for _, taint := range taints {
		resp = append(resp, types.NodeTaint{
			Key:      taint.Key,
			Value:    taint.Value,
			Effect:   taint.Effect,
			Time:     parseTaintTime(taint.Time),
			IsDelete: taint.IsDelete, // 系统污点不可删除
		})
	}

	return resp, nil
}

func parseTaintTime(timeStr string) int64 {
	if timeStr == "" {
		return 0
	}
	// 这里根据实际的时间格式进行解析
	// 例如，如果是 RFC3339 格式
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return 0
	}
	return t.Unix()
}
