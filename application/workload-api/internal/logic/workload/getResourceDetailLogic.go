package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取资源详情信息
func NewGetResourceDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceDetailLogic {
	return &GetResourceDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceDetailLogic) GetResourceDetail(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}
	resourceType := strings.ToUpper(versionDetail.ResourceType)
	switch resourceType {
	case "DEPLOYMENT":
		deploymentOperator := client.Deployment()
		describe, err := deploymentOperator.GetDescribe(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取资源详情失败: %v", err)
			return "", err
		}
		return describe, nil
	case "STATEFULSET":
		statefulsetOperator := client.StatefulSet()
		describe, err := statefulsetOperator.GetDescribe(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取资源详情失败: %v", err)
			return "", err
		}
		return describe, nil
	case "DAEMONSET":
		daemonsetOperator := client.DaemonSet()
		describe, err := daemonsetOperator.GetDescribe(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取资源详情失败: %v", err)
			return "", err
		}
		return describe, nil
	case "JOB":
		jobOperator := client.Job()
		describe, err := jobOperator.GetDescribe(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取资源详情失败: %v", err)
			return "", err
		}
		return describe, nil
	case "CRONJOB":
		cronjobOperator := client.CronJob()
		describe, err := cronjobOperator.GetDescribe(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取资源详情失败: %v", err)
			return "", err
		}
		return describe, nil
	case "POD":
		podOperator := client.Pods()
		describe, err := podOperator.GetDescribe(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取资源详情失败: %v", err)
			return "", err
		}
		return describe, nil
	default:
		l.Errorf("不支持的资源类型: %s", resourceType)
		return "", fmt.Errorf("不支持的资源类型: %s", resourceType)

	}
}
