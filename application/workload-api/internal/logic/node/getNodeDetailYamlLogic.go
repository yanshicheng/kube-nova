package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeDetailYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeDetailYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeDetailYamlLogic {
	return &GetNodeDetailYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeDetailYamlLogic) GetNodeDetailYaml(req *types.DefaultNodeNameRequest) (resp string, err error) {
	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取节点详情 - 使用新的 GetNodeDetail 方法
	nodeOperator := client.Node()
	detail, err := nodeOperator.Describe(req.NodeName)
	if err != nil {
		l.Errorf("获取节点详情失败: %v", err)
		return "", fmt.Errorf("获取节点详情失败")
	}
	return detail, nil
}
