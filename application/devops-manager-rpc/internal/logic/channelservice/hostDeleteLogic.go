package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostDeleteLogic {
	return &HostDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostDeleteLogic) HostDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	if err := l.svcCtx.HostModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("主机删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
