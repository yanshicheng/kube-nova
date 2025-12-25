package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetVersionLabelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取某一个版本的 labels
func NewGetVersionLabelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVersionLabelsLogic {
	return &GetVersionLabelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetVersionLabelsLogic) GetVersionLabels(req *types.DefaultIdRequest) (resp []types.VersionLabelsResp, err error) {

	version, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: req.Id,
	})
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, version.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}
	labels := make(map[string]string)
	switch strings.ToLower(version.ResourceType) {
	case "deployment":
		labels, err = client.Deployment().GetPodLabels(version.Namespace, version.ResourceName)
		if err != nil {
			l.Errorf("获取 deployment labels 失败: %v", err)
			return nil, fmt.Errorf("获取 deployment labels 失败")
		}
	case "statefulset":
		labels, err = client.StatefulSet().GetPodLabels(version.Namespace, version.ResourceName)
		if err != nil {
			l.Errorf("获取 statefulset labels 失败: %v", err)
			return nil, fmt.Errorf("获取 statefulset labels 失败")
		}
	case "daemonset":
		labels, err = client.DaemonSet().GetPodLabels(version.Namespace, version.ResourceName)
		if err != nil {
			l.Errorf("获取 daemonset labels 失败: %v", err)
			return nil, fmt.Errorf("获取 daemonset labels 失败")
		}
	default:
		return nil, fmt.Errorf("不支持的资源类型")

	}
	for key, value := range labels {
		resp = append(resp, types.VersionLabelsResp{
			Key:   key,
			Value: value,
		})
	}
	return
}
