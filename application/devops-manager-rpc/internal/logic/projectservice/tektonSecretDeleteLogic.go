package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretDeleteLogic {
	return &TektonSecretDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretDeleteLogic) TektonSecretDelete(in *pb.DeleteTektonSecretReq) (*pb.EmptyResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析项目 Tekton Secret 渠道失败: %v", err)
		return nil, err
	}
	if err := ref.Client.DeleteProjectSecret(l.ctx, ref.BindingConfig.Namespace, in.Name, in.ProjectId, ref.Binding.ID.Hex()); err != nil {
		l.Errorf("删除项目 Tekton Secret 失败: %v", err)
		return nil, tektonSecretClientError(err)
	}

	return &pb.EmptyResp{}, nil
}
