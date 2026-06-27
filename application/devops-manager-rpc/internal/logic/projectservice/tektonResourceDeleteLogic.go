package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceDeleteLogic {
	return &TektonResourceDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceDeleteLogic) TektonResourceDelete(in *pb.DeleteTektonResourceReq) (*pb.EmptyResp, error) {
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
		if err := ref.Client.DeleteConfigMap(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("删除 Tekton ConfigMap 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "pvc":
		if err := ref.Client.DeletePersistentVolumeClaim(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("删除 Tekton PVC 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "serviceAccount":
		if err := ref.Client.DeleteServiceAccount(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("删除 Tekton ServiceAccount 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "secret":
		if err := ref.Client.DeleteProjectSecret(l.ctx, ref.BindingConfig.Namespace, name, in.ProjectId, ref.Binding.ID.Hex()); err != nil {
			l.Errorf("删除 Tekton Secret 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "pod":
		if err := ref.Client.DeletePod(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("删除 Tekton Pod 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "pipeline":
		if err := ref.Client.DeletePipeline(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("删除 Tekton Pipeline 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "pipelineRun":
		if err := ref.Client.DeletePipelineRun(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("删除 Tekton PipelineRun 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	default:
		return nil, errorx.Msg("Tekton 资源类型不支持")
	}
}
