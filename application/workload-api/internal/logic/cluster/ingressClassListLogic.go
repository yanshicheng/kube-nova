package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 IngressClass 列表
func NewIngressClassListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassListLogic {
	return &IngressClassListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassListLogic) IngressClassList(req *types.IngressClassListRequest) (resp *types.IngressClassListResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	result, err := icOp.List(req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 IngressClass 列表失败: %v", err)
		return nil, fmt.Errorf("获取 IngressClass 列表失败")
	}

	resp = &types.IngressClassListResponse{
		Total: result.Total,
		Items: make([]types.IngressClassListItem, 0, len(result.Items)),
	}

	for _, item := range result.Items {
		resp.Items = append(resp.Items, types.IngressClassListItem{
			Name:              item.Name,
			Controller:        item.Controller,
			Parameters:        item.Parameters,
			IsDefault:         item.IsDefault,
			Age:               item.Age,
			CreationTimestamp: item.CreationTimestamp,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
		})
	}

	return resp, nil
}
