package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretGetLogic {
	return &TektonSecretGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretGetLogic) TektonSecretGet(in *pb.GetTektonSecretReq) (*pb.GetTektonSecretResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析项目 Tekton Secret 渠道失败: %v", err)
		return nil, err
	}
	secret, err := ref.Client.GetProjectSecret(l.ctx, ref.BindingConfig.Namespace, in.Name, in.ProjectId, ref.Binding.ID.Hex())
	if err != nil {
		l.Errorf("读取项目 Tekton Secret 失败: %v", err)
		return nil, tektonSecretClientError(err)
	}

	return &pb.GetTektonSecretResp{Data: tektonSecretToPb(secret, ref, true)}, nil
}
