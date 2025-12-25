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

type GetResourceHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetResourceHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceHistoryLogic {
	return &GetResourceHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceHistoryLogic) GetResourceHistory(req *types.DefaultIdRequest) (resp []types.RevisionInfo, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		revisions, err := client.Deployment().GetRevisions(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Deployment 版本历史失败: %v", err)
			return nil, fmt.Errorf("获取版本历史失败")
		}
		resp = convertToRevisionInfo(revisions)
	case "DAEMONSET":
		revisions, err := client.DaemonSet().GetRevisions(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 DaemonSet 版本历史失败: %v", err)
			return nil, fmt.Errorf("获取版本历史失败")
		}
		resp = convertToRevisionInfo(revisions)
	case "STATEFULSET":
		revisions, err := client.StatefulSet().GetRevisions(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 StatefulSet 版本历史失败: %v", err)
			return nil, fmt.Errorf("获取版本历史失败")
		}
		resp = convertToRevisionInfo(revisions)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询版本历史", resourceType)
	}

	return resp, nil
}

func convertToRevisionInfo(revisions []types2.RevisionInfo) []types.RevisionInfo {
	result := make([]types.RevisionInfo, 0, len(revisions))
	for _, rev := range revisions {
		result = append(result, types.RevisionInfo{
			Revision:          rev.Revision,
			CreationTimestamp: rev.CreationTimestamp,
			Images:            rev.Images,
			Replicas:          rev.Replicas,
			Reason:            rev.Reason,
		})
	}
	return result
}
