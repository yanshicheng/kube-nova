package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceListLogic {
	return &TektonResourceListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceListLogic) TektonResourceList(in *pb.ListTektonResourceReq) (*pb.ListTektonResourceResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析 Tekton 资源绑定失败: %v", err)
		return nil, err
	}
	resourceType := normalizeTektonResourceType(in.ResourceType)
	switch resourceType {
	case "configMap":
		items, err := ref.Client.ListConfigMaps(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton ConfigMap 失败: %v", err)
			return nil, err
		}
		data := configMapsToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	case "pvc":
		items, err := ref.Client.ListPersistentVolumeClaims(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton PVC 失败: %v", err)
			return nil, err
		}
		data := pvcsToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	case "serviceAccount":
		items, err := ref.Client.ListServiceAccounts(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton ServiceAccount 失败: %v", err)
			return nil, err
		}
		data := serviceAccountsToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	case "secret":
		items, err := ref.Client.ListProjectSecrets(l.ctx, ref.BindingConfig.Namespace, in.Keyword, in.ProjectId, ref.Binding.ID.Hex())
		if err != nil {
			l.Errorf("查询 Tekton Secret 失败: %v", err)
			return nil, err
		}
		data := secretsToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	case "pod":
		items, err := ref.Client.ListPods(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton Pod 失败: %v", err)
			return nil, err
		}
		data := podsToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	case "pipeline":
		items, err := ref.Client.ListPipelines(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton Pipeline 失败: %v", err)
			return nil, err
		}
		data := pipelinesToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	case "pipelineRun":
		items, err := ref.Client.ListPipelineRuns(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
		if err != nil {
			l.Errorf("查询 Tekton PipelineRun 失败: %v", err)
			return nil, err
		}
		data := pipelineRunsToResource(items, ref, resourceType)
		return &pb.ListTektonResourceResp{Data: data, Total: uint64(len(data))}, nil
	default:
		return nil, errorx.Msg("Tekton 资源类型不支持")
	}
}
