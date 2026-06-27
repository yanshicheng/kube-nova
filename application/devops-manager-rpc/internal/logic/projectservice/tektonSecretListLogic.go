package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretListLogic {
	return &TektonSecretListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretListLogic) TektonSecretList(in *pb.ListTektonSecretReq) (*pb.ListTektonSecretResp, error) {
	bindingID := in.BindingId
	if bindingID == "" {
		refs, err := listTektonSecretBindingRefs(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("查询项目 Tekton Secret 渠道失败: %v", err)
			return nil, err
		}
		if len(refs) == 0 {
			return &pb.ListTektonSecretResp{}, nil
		}
		bindingID = refs[0].Binding.ID.Hex()
	}
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, bindingID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析项目 Tekton Secret 渠道失败: %v", err)
		return nil, err
	}
	secrets, err := ref.Client.ListProjectSecrets(l.ctx, ref.BindingConfig.Namespace, in.Keyword, in.ProjectId, ref.Binding.ID.Hex())
	if err != nil {
		l.Errorf("查询项目 Tekton Secret 列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.TektonSecret, 0, len(secrets))
	for _, item := range secrets {
		items = append(items, tektonSecretInfoToPb(item, ref))
	}

	return &pb.ListTektonSecretResp{Data: items, Total: uint64(len(items))}, nil
}
