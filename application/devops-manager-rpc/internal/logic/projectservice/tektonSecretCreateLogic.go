package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretCreateLogic {
	return &TektonSecretCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretCreateLogic) TektonSecretCreate(in *pb.SaveTektonSecretReq) (*pb.EmptyResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析项目 Tekton Secret 渠道失败: %v", err)
		return nil, err
	}
	secret, err := buildKubernetesSecret(in, ref.BindingConfig.Namespace)
	if err != nil {
		l.Errorf("构建项目 Tekton Secret 失败: %v", err)
		return nil, err
	}
	if err := ref.Client.EnsureNamespace(l.ctx, ref.BindingConfig.Namespace); err != nil {
		l.Errorf("创建项目 Tekton Secret namespace 失败: %v", err)
		return nil, tektonSecretClientError(err)
	}
	if err := ref.Client.CreateSecret(l.ctx, ref.BindingConfig.Namespace, secret); err != nil {
		l.Errorf("创建项目 Tekton Secret 失败: %v", err)
		return nil, tektonSecretClientError(err)
	}

	return &pb.EmptyResp{}, nil
}
