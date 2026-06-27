package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretBindingListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretBindingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretBindingListLogic {
	return &TektonSecretBindingListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretBindingListLogic) TektonSecretBindingList(in *pb.ListTektonSecretBindingReq) (*pb.ListTektonSecretBindingResp, error) {
	refs, err := listTektonSecretBindingRefs(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("查询项目 Tekton Secret 渠道失败: %v", err)
		return nil, err
	}
	items := make([]*pb.TektonSecretBindingOption, 0, len(refs))
	for _, ref := range refs {
		items = append(items, tektonSecretBindingOption(ref))
	}

	return &pb.ListTektonSecretBindingResp{Data: items, Total: uint64(len(items))}, nil
}
