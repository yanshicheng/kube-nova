package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUpdateStrategyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUpdateStrategyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUpdateStrategyLogic {
	return &GetUpdateStrategyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUpdateStrategyLogic) GetUpdateStrategy(req *types.DefaultIdRequest) (resp *types.UpdateStrategyResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var strategy *types2.UpdateStrategyResponse

	switch resourceType {
	case "DEPLOYMENT":
		strategy, err = client.Deployment().GetUpdateStrategy(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		strategy, err = client.StatefulSet().GetUpdateStrategy(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		strategy, err = client.DaemonSet().GetUpdateStrategy(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询更新策略", resourceType)
	}

	if err != nil {
		l.Errorf("获取更新策略失败: %v", err)
		return nil, fmt.Errorf("获取更新策略失败")
	}

	resp = convertToUpdateStrategyResponse(strategy)
	return resp, nil
}

func convertToUpdateStrategyResponse(strategy *types2.UpdateStrategyResponse) *types.UpdateStrategyResponse {
	resp := &types.UpdateStrategyResponse{
		Type: strategy.Type,
	}

	if strategy.RollingUpdate != nil {
		resp.RollingUpdate = &types.RollingUpdateConfig{
			MaxUnavailable: strategy.RollingUpdate.MaxUnavailable,
			MaxSurge:       strategy.RollingUpdate.MaxSurge,
			Partition:      strategy.RollingUpdate.Partition,
		}
	}

	return resp
}
