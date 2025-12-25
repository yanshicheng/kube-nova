package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceLabelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取资源的labels
func NewGetResourceLabelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceLabelsLogic {
	return &GetResourceLabelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceLabelsLogic) GetResourceLabels(req *types.DefaultIdRequest) (resp *types.PodLabelsResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var labels map[string]string

	switch resourceType {
	case "DEPLOYMENT":
		labels, err = client.Deployment().GetPodLabels(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		labels, err = client.StatefulSet().GetPodLabels(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		labels, err = client.DaemonSet().GetPodLabels(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		labels, err = client.Job().GetPodLabels(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		labels, err = client.CronJob().GetPodLabels(versionDetail.Namespace, versionDetail.ResourceName)
	case "POD":
		labels, err = client.Pods().GetPodLabels(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询标签", resourceType)
	}

	if err != nil {
		l.Errorf("获取资源标签失败: %v", err)
		return nil, fmt.Errorf("获取资源标签失败")
	}

	resp = &types.PodLabelsResponse{
		Labels: labels,
	}

	l.Infof("成功获取资源标签，命名空间: %s, 资源名称: %s, 标签数量: %d",
		versionDetail.Namespace, versionDetail.ResourceName, len(labels))

	return resp, nil
}
