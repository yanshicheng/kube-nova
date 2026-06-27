package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceSaveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceSaveLogic {
	return &TektonResourceSaveLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceSaveLogic) TektonResourceSave(in *pb.SaveTektonResourceReq) (*pb.EmptyResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析 Tekton 资源绑定失败: %v", err)
		return nil, err
	}
	resourceType := normalizeTektonResourceType(in.ResourceType)
	switch resourceType {
	case "configMap":
		cm, err := parseConfigMap(in.Yaml)
		if err != nil {
			l.Errorf("解析 Tekton ConfigMap 失败: %v", err)
			return nil, err
		}
		if err := validateTektonResourceName(in.Name, cm.Name); err != nil {
			return nil, err
		}
		applyTektonResourceLabels(&cm.ObjectMeta, in.ProjectId, ref.Binding.ID.Hex(), resourceType)
		if err := ref.Client.ApplyConfigMap(l.ctx, ref.BindingConfig.Namespace, cm); err != nil {
			l.Errorf("保存 Tekton ConfigMap 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "pvc":
		pvc, err := parsePVC(in.Yaml)
		if err != nil {
			l.Errorf("解析 Tekton PVC 失败: %v", err)
			return nil, err
		}
		if err := validateTektonResourceName(in.Name, pvc.Name); err != nil {
			return nil, err
		}
		applyTektonResourceLabels(&pvc.ObjectMeta, in.ProjectId, ref.Binding.ID.Hex(), resourceType)
		if err := ref.Client.ApplyPersistentVolumeClaim(l.ctx, ref.BindingConfig.Namespace, pvc); err != nil {
			l.Errorf("保存 Tekton PVC 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "serviceAccount":
		sa, err := parseServiceAccount(in.Yaml)
		if err != nil {
			l.Errorf("解析 Tekton ServiceAccount 失败: %v", err)
			return nil, err
		}
		if err := validateTektonResourceName(in.Name, sa.Name); err != nil {
			return nil, err
		}
		applyTektonResourceLabels(&sa.ObjectMeta, in.ProjectId, ref.Binding.ID.Hex(), resourceType)
		if err := ref.Client.ApplyServiceAccount(l.ctx, ref.BindingConfig.Namespace, sa); err != nil {
			l.Errorf("保存 Tekton ServiceAccount 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	case "secret":
		secret, err := parseSecret(in.Yaml, in.ProjectId, ref.Binding.ID.Hex())
		if err != nil {
			l.Errorf("解析 Tekton Secret 失败: %v", err)
			return nil, err
		}
		if err := validateTektonResourceName(in.Name, secret.Name); err != nil {
			return nil, err
		}
		if err := ref.Client.ApplyProjectSecret(l.ctx, ref.BindingConfig.Namespace, secret); err != nil {
			l.Errorf("保存 Tekton Secret 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	default:
		return nil, errorx.Msg("Tekton 资源类型不支持")
	}
}
