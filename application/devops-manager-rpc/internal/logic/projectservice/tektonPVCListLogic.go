package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonPVCListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonPVCListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonPVCListLogic {
	return &TektonPVCListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonPVCListLogic) TektonPVCList(in *pb.ListTektonPVCReq) (*pb.ListTektonPVCResp, error) {
	bindingID := in.BindingId
	if bindingID == "" {
		refs, err := listTektonSecretBindingRefs(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("查询项目 Tekton PVC 渠道失败: %v", err)
			return nil, err
		}
		if len(refs) == 0 {
			return &pb.ListTektonPVCResp{}, nil
		}
		bindingID = refs[0].Binding.ID.Hex()
	}
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, bindingID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析项目 Tekton PVC 渠道失败: %v", err)
		return nil, err
	}
	pvcs, err := ref.Client.ListPersistentVolumeClaims(l.ctx, ref.BindingConfig.Namespace, in.Keyword)
	if err != nil {
		l.Errorf("查询项目 Tekton PVC 列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.TektonPVC, 0, len(pvcs))
	for _, item := range pvcs {
		items = append(items, tektonPVCInfoToPb(item, ref))
	}

	return &pb.ListTektonPVCResp{Data: items, Total: uint64(len(items))}, nil
}
