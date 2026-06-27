package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretUpdateLogic {
	return &TektonSecretUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretUpdateLogic) TektonSecretUpdate(in *pb.SaveTektonSecretReq) (*pb.EmptyResp, error) {
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
	if err := ref.Client.UpdateSecret(l.ctx, ref.BindingConfig.Namespace, secret); err != nil {
		l.Errorf("更新项目 Tekton Secret 失败: %v", err)
		return nil, tektonSecretClientError(err)
	}

	return &pb.EmptyResp{}, nil
}
