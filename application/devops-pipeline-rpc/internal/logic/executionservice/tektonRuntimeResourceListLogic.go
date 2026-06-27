package executionservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonRuntimeResourceListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonRuntimeResourceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonRuntimeResourceListLogic {
	return &TektonRuntimeResourceListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonRuntimeResourceListLogic) TektonRuntimeResourceList(in *pb.ListTektonRuntimeResourceReq) (*pb.ListTektonRuntimeResourceResp, error) {
	runtime, err := buildRuntime(l.ctx, l.svcCtx, in.ProjectId, in.SystemId, in.EnvironmentId, in.BuildChannelBindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("查询 Tekton 运行时资源失败: %v", err)
		return nil, err
	}
	if runtime == nil || runtime.Binding == nil || runtime.Channel == nil {
		l.Errorf("查询 Tekton 运行时资源失败: Tekton 运行时配置不完整")
		return nil, errorx.Msg("Tekton 运行时配置不完整")
	}
	if runtime.Binding.ChannelType != engineTekton || runtime.Channel.ChannelType != engineTekton {
		return nil, errorx.Msg("构建渠道不是 Tekton 类型")
	}
	bindingCfg, err := parseTektonBindingConfig(runtime.Binding.BindingConfig)
	if err != nil {
		l.Errorf("查询 Tekton 运行时资源失败: %v", err)
		return nil, err
	}
	channelCfg, err := parseTektonChannelTaskConfig(runtime.Channel.Config)
	if err != nil {
		l.Errorf("查询 Tekton 运行时资源失败: %v", err)
		return nil, errorx.Msg(err.Error())
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		l.Errorf("创建 Tekton 客户端失败: %v", err)
		return nil, err
	}

	var data []*pb.TektonRuntimeResource
	switch strings.TrimSpace(in.ResourceType) {
	case "task":
		items, err := client.ListTasks(l.ctx, channelCfg.TaskNamespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton Task 失败: %v", err)
			return nil, err
		}
		data = tektonTaskInfosToPb(items)
	case "clusterTask":
		items, err := client.ListClusterTasks(l.ctx, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton ClusterTask 失败: %v", err)
			return nil, err
		}
		data = tektonTaskInfosToPb(items)
	case "serviceAccount":
		items, err := client.ListServiceAccounts(l.ctx, bindingCfg.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Kubernetes ServiceAccount 失败: %v", err)
			return nil, err
		}
		data = tektonServiceAccountsToPb(items)
	case "storageClass":
		items, err := client.ListStorageClasses(l.ctx, in.Keyword)
		if err != nil {
			l.Errorf("查询 Kubernetes StorageClass 失败: %v", err)
			return nil, err
		}
		data = tektonStorageClassesToPb(items)
	case "configMap":
		items, err := client.ListConfigMaps(l.ctx, bindingCfg.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Kubernetes ConfigMap 失败: %v", err)
			return nil, err
		}
		data = tektonConfigMapsToPb(items)
	default:
		return nil, errorx.Msg("Tekton 运行时资源类型不支持")
	}
	return &pb.ListTektonRuntimeResourceResp{Data: data, Total: uint64(len(data))}, nil
}

func tektonTaskInfosToPb(items []devopstekton.TaskInfo) []*pb.TektonRuntimeResource {
	result := make([]*pb.TektonRuntimeResource, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.TektonRuntimeResource{
			Name:        item.Name,
			Namespace:   item.Namespace,
			Kind:        item.Kind,
			ApiVersion:  item.APIVersion,
			Description: item.Description,
			Params:      item.Params,
			Workspaces:  item.Workspaces,
			Results:     item.Results,
			CreatedAt:   item.CreatedAt,
			Yaml:        item.Yaml,
		})
	}
	return result
}

func tektonServiceAccountsToPb(items []devopstekton.ServiceAccountInfo) []*pb.TektonRuntimeResource {
	result := make([]*pb.TektonRuntimeResource, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.TektonRuntimeResource{
			Name:             item.Name,
			Namespace:        item.Namespace,
			Kind:             "ServiceAccount",
			ApiVersion:       "v1",
			Secrets:          item.Secrets,
			ImagePullSecrets: item.ImagePullSecrets,
			CreatedAt:        item.CreatedAt,
		})
	}
	return result
}

func tektonStorageClassesToPb(items []devopstekton.StorageClassInfo) []*pb.TektonRuntimeResource {
	result := make([]*pb.TektonRuntimeResource, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.TektonRuntimeResource{
			Name:                 item.Name,
			Kind:                 "StorageClass",
			ApiVersion:           "storage.k8s.io/v1",
			Provisioner:          item.Provisioner,
			ReclaimPolicy:        item.ReclaimPolicy,
			VolumeBindingMode:    item.VolumeBindingMode,
			AllowVolumeExpansion: item.AllowVolumeExpansion,
			CreatedAt:            item.CreatedAt,
		})
	}
	return result
}

func tektonConfigMapsToPb(items []devopstekton.ConfigMapInfo) []*pb.TektonRuntimeResource {
	result := make([]*pb.TektonRuntimeResource, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.TektonRuntimeResource{
			Name:       item.Name,
			Namespace:  item.Namespace,
			Kind:       "ConfigMap",
			ApiVersion: "v1",
			Keys:       item.Keys,
			CreatedAt:  item.CreatedAt,
		})
	}
	return result
}
