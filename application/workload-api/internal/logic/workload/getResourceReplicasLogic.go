package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceReplicasLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询资源副本数
func NewGetResourceReplicasLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceReplicasLogic {
	return &GetResourceReplicasLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceReplicasLogic) GetResourceReplicas(req *types.DefaultIdRequest) (resp int32, err error) {

	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return 0, fmt.Errorf("获取集群客户端失败")
	}
	switch strings.ToUpper(versionDetail.ResourceType) {
	case "DEPLOYMENT":

		replicas, err := client.Deployment().GetReplicas(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取Deployment副本数失败: %v", err)
			return 0, fmt.Errorf("获取Deployment副本数失败")
		}
		return replicas.Replicas, nil
	case "STATEFULSET":
		replicas, err := client.StatefulSet().GetReplicas(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取StatefulSet副本数失败: %v", err)
			return 0, fmt.Errorf("获取StatefulSet副本数失败")
		}
		return replicas.Replicas, nil
	default:
		return 0, fmt.Errorf("不支持的资源类型")
	}
}
