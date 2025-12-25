package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetApplicationSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取应用摘要
func NewGetApplicationSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetApplicationSummaryLogic {
	return &GetApplicationSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetApplicationSummaryLogic) GetApplicationSummary(req *types.GetApplicationSummaryRequest) (resp *types.ApplicationSummary, err error) {
	// 获取应用
	app, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: req.ApplicationId,
	})
	if err != nil {
		l.Errorf("获取应用失败: %v", err)
		return nil, fmt.Errorf("获取应用失败: %v", err)
	}

	// 获取应用下所有的版本
	versions, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: req.ApplicationId,
	})
	if err != nil {
		l.Errorf("获取应用版本失败: %v", err)
		return nil, fmt.Errorf("获取应用版本失败: %v", err)
	}

	// 如果没有版本，返回空摘要
	if len(versions.Data) == 0 {
		l.Infof("应用 %d 没有版本", req.ApplicationId)
		return &types.ApplicationSummary{
			PodCount:         0,
			AbnormalPodCount: 0,
			ServiceCount:     0,
			IngressCount:     0,
			Service: types.ApplicationAccessSummary{
				InternalAccessList: []string{},
				ExternalAccessList: []string{},
				NodePortList:       []string{},
			},
			IngressDomains: []string{},
		}, nil
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}

	domainSuffix := ""
	// 获取集群配置
	domainData, err := l.svcCtx.ManagerRpc.GetClusterDefaultDomainSuffix(l.ctx, &managerservice.GetClusterDefaultDomainSuffixReq{
		Uuid: req.ClusterUuid,
	})
	if err != nil {
		l.Errorf("获取集群默认域名后缀失败: %v", err)
		// 域名后缀失败不影响继续执行，使用空字符串
	}
	domainSuffix = domainData.Data
	var nodeLb []string
	nodeLbData, err := l.svcCtx.ManagerRpc.GetClusterLbList(l.ctx, &managerservice.GetClusterLbListReq{
		Uuid:   req.ClusterUuid,
		LbName: "node",
	})
	if err != nil {
		l.Errorf("获取集群 LB 列表失败: %v", err)
	}
	nodeLb = nodeLbData.Data
	// 获取操作器
	podOp := client.Pods()
	serviceOp := client.Services()
	ingressOp := client.Ingresses()

	// 收集所有版本的摘要
	versionSummaries := make([]*k8sTypes.WorkloadResourceSummary, 0, len(versions.Data))

	// 根据资源类型获取摘要
	resourceType := strings.ToLower(app.Data.ResourceType)

	for _, version := range versions.Data {
		var summary *k8sTypes.WorkloadResourceSummary
		var err error

		switch resourceType {
		case "deployment":
			summary, err = client.Deployment().GetResourceSummary(
				req.Namespace,
				version.ResourceName,
				domainSuffix,
				nodeLb,
				podOp,
				serviceOp,
				ingressOp,
			)

		case "statefulset":
			summary, err = client.StatefulSet().GetResourceSummary(
				req.Namespace,
				version.ResourceName,
				domainSuffix,
				nodeLb,
				podOp,
				serviceOp,
				ingressOp,
			)

		case "daemonset":
			summary, err = client.DaemonSet().GetResourceSummary(
				req.Namespace,
				version.ResourceName,
				domainSuffix,
				nodeLb,
				podOp,
				serviceOp,
				ingressOp,
			)

		case "cronjob":
			summary, err = client.CronJob().GetResourceSummary(
				req.Namespace,
				version.ResourceName,
				domainSuffix,
				nodeLb,
				podOp,
				serviceOp,
				ingressOp,
			)

		default:
			l.Errorf("不支持的资源类型: %s", app.Data.ResourceType)
			return nil, fmt.Errorf("资源类型 %s 不支持获取应用摘要", app.Data.ResourceType)
		}

		if err != nil {
			l.Errorf("获取版本 %s 的资源摘要失败: %v", version.ResourceName, err)
			// 单个版本失败不影响其他版本，继续处理
			continue
		}

		l.Infof("版本 %s 摘要: PodCount=%d, ServiceCount=%d, IsAppSelector=%v",
			version.ResourceName, summary.PodCount, summary.ServiceCount, summary.IsAppSelector)

		versionSummaries = append(versionSummaries, summary)
	}

	// 如果所有版本都失败了
	if len(versionSummaries) == 0 {
		l.Errorf("所有版本的资源摘要获取都失败了")
		return nil, fmt.Errorf("获取应用摘要失败: 所有版本都失败")
	}

	// 合并所有版本的摘要
	applicationSummary := MergeApplicationSummaries(versionSummaries, len(versions.Data))

	l.Infof("应用 %d 最终摘要: PodCount=%d, ServiceCount=%d, IngressCount=%d",
		req.ApplicationId, applicationSummary.PodCount, applicationSummary.ServiceCount, applicationSummary.IngressCount)

	return applicationSummary, nil
}

// MergeApplicationSummaries 合并多个版本的资源摘要为应用级别摘要
func MergeApplicationSummaries(summaries []*k8sTypes.WorkloadResourceSummary, versionTotal int) *types.ApplicationSummary {
	if len(summaries) == 0 {
		return &types.ApplicationSummary{
			Service: types.ApplicationAccessSummary{
				InternalAccessList: []string{},
				ExternalAccessList: []string{},
				NodePortList:       []string{},
			},
			IngressDomains: []string{},
		}
	}

	result := &types.ApplicationSummary{
		PodCount:         0,
		AbnormalPodCount: 0,
		ServiceCount:     0,
		IngressCount:     0,
		Service: types.ApplicationAccessSummary{
			InternalAccessList: []string{},
			ExternalAccessList: []string{},
			NodePortList:       []string{},
		},
		IngressDomains: []string{},
	}

	// 用于去重的 map
	internalAccessSet := make(map[string]bool)
	externalAccessSet := make(map[string]bool)
	nodePortSet := make(map[string]bool)
	ingressDomainSet := make(map[string]bool)

	// 标记是否已经处理过 IsAppSelector 的减 1 操作
	hasProcessedAppSelector := false

	// 遍历所有版本的摘要
	for _, summary := range summaries {
		if summary == nil {
			continue
		}

		result.PodCount += summary.PodCount
		result.AbnormalPodCount += summary.AbnormalPodCount

		result.ServiceCount += summary.ServiceCount

		if versionTotal != 1 {
			if summary.IsAppSelector && !hasProcessedAppSelector {
				result.ServiceCount--
				hasProcessedAppSelector = true
			}
		}

		result.IngressCount += summary.IngressCount

		for _, addr := range summary.Service.InternalAccessList {
			if addr != "" && !internalAccessSet[addr] {
				result.Service.InternalAccessList = append(result.Service.InternalAccessList, addr)
				internalAccessSet[addr] = true
			}
		}

		for _, addr := range summary.Service.ExternalAccessList {
			if addr != "" && !externalAccessSet[addr] {
				result.Service.ExternalAccessList = append(result.Service.ExternalAccessList, addr)
				externalAccessSet[addr] = true
			}
		}

		for _, addr := range summary.Service.NodePortList {
			if addr != "" && !nodePortSet[addr] {
				result.Service.NodePortList = append(result.Service.NodePortList, addr)
				nodePortSet[addr] = true
			}
		}

		for _, domain := range summary.IngressDomains {
			if domain != "" && !ingressDomainSet[domain] {
				result.IngressDomains = append(result.IngressDomains, domain)
				ingressDomainSet[domain] = true
			}
		}
	}

	return result
}
