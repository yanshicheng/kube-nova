package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceGetLogic {
	return &TektonResourceGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceGetLogic) TektonResourceGet(in *pb.GetTektonResourceReq) (*pb.GetTektonResourceResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析 Tekton 资源绑定失败: %v", err)
		return nil, err
	}
	resourceType := normalizeTektonResourceType(in.ResourceType)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errorx.Msg("资源名称不能为空")
	}
	switch resourceType {
	case "configMap":
		cm, err := ref.Client.GetConfigMap(l.ctx, ref.BindingConfig.Namespace, name)
		if err != nil {
			l.Errorf("获取 Tekton ConfigMap 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: configMapToResourceDetail(cm, ref, resourceType)}, nil
	case "pvc":
		pvc, err := ref.Client.GetPersistentVolumeClaim(l.ctx, ref.BindingConfig.Namespace, name)
		if err != nil {
			l.Errorf("获取 Tekton PVC 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: pvcToResourceDetail(pvc, ref, resourceType)}, nil
	case "serviceAccount":
		sa, err := ref.Client.GetServiceAccount(l.ctx, ref.BindingConfig.Namespace, name)
		if err != nil {
			l.Errorf("获取 Tekton ServiceAccount 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: serviceAccountToResourceDetail(sa, ref, resourceType)}, nil
	case "secret":
		secret, err := ref.Client.GetProjectSecret(l.ctx, ref.BindingConfig.Namespace, name, in.ProjectId, ref.Binding.ID.Hex())
		if err != nil {
			l.Errorf("获取 Tekton Secret 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: secretToResourceDetail(secret, ref, resourceType)}, nil
	case "pod":
		content, err := ref.Client.GetPodYaml(l.ctx, ref.BindingConfig.Namespace, name)
		if err != nil {
			l.Errorf("获取 Tekton Pod 失败: %v", err)
			return nil, err
		}
		item, err := podFromYAML(content)
		if err != nil {
			l.Errorf("解析 Tekton Pod 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: tektonResourceToPb(item, ref, resourceType)}, nil
	case "pipeline":
		content, err := ref.Client.GetPipelineYaml(l.ctx, ref.BindingConfig.Namespace, name)
		if err != nil {
			l.Errorf("获取 Tekton Pipeline 失败: %v", err)
			return nil, err
		}
		item, err := pipelineFromYAML(content)
		if err != nil {
			l.Errorf("解析 Tekton Pipeline 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: tektonResourceToPb(item, ref, resourceType)}, nil
	case "pipelineRun":
		content, err := ref.Client.GetPipelineRunYaml(l.ctx, ref.BindingConfig.Namespace, name)
		if err != nil {
			l.Errorf("获取 Tekton PipelineRun 失败: %v", err)
			return nil, err
		}
		item, err := pipelineRunFromYAML(content)
		if err != nil {
			l.Errorf("解析 Tekton PipelineRun 失败: %v", err)
			return nil, err
		}
		return &pb.GetTektonResourceResp{Data: tektonResourceToPb(item, ref, resourceType)}, nil
	default:
		return nil, errorx.Msg("Tekton 资源类型不支持")
	}
}
