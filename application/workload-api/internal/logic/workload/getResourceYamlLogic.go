package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取资源yaml
func NewGetResourceYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceYamlLogic {
	return &GetResourceYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceYamlLogic) GetResourceYaml(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}
	switch strings.ToUpper(versionDetail.ResourceType) {
	case "DEPLOYMENT":
		resp, err = client.Deployment().GetYaml(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Deployment 获取 yaml 失败: %v", err)
			return "", fmt.Errorf("获取 yaml 失败")
		}
	case "POD":
		resp, err = client.Pods().GetYaml(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Pod 获取 yaml 失败: %v", err)
			return "", fmt.Errorf("获取 yaml 失败")
		}
	case "DAEMONSET":
		resp, err = client.DaemonSet().GetYaml(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("DaemonSet 获取 yaml 失败: %v", err)
			return "", fmt.Errorf("获取 yaml 失败")
		}
	case "JOB":
		resp, err = client.Job().GetYaml(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Job 获取 yaml 失败: %v", err)
			return "", fmt.Errorf("获取 yaml 失败")
		}
	case "CRONJOB":
		resp, err = client.CronJob().GetYaml(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("CronJob 获取 yaml 失败: %v", err)
			return "", fmt.Errorf("获取 yaml 失败")
		}
	case "STATEFULSET":
		resp, err = client.StatefulSet().GetYaml(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("StatefulSet 获取 yaml 失败: %v", err)
			return "", fmt.Errorf("获取 yaml 失败")
		}
	default:
		return "", fmt.Errorf("不支持的资源类型")
	}
	return
}
