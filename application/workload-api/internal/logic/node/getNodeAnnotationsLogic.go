package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeAnnotationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeAnnotationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeAnnotationsLogic {
	return &GetNodeAnnotationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeAnnotationsLogic) GetNodeAnnotations(req *types.DefaultNodeNameRequest) (resp []types.NodeAnnotationItem, err error) {
	// 1. 参数验证
	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取节点注解
	nodeOperator := client.Node()
	annotations, err := nodeOperator.GetAnnotations(req.NodeName)
	if err != nil {
		l.Errorf("获取节点注解失败: %v", err)
		return nil, fmt.Errorf("获取节点注解失败")
	}

	// 4. 转换为响应格式，并标记系统注解不可删除
	resp = make([]types.NodeAnnotationItem, 0, len(annotations))
	for _, annotation := range annotations {
		resp = append(resp, types.NodeAnnotationItem{
			Key:      annotation.Key,
			Value:    annotation.Value,
			IsDelete: annotation.IsDelete, // 系统注解不可删除
		})
	}

	return resp, nil
}
