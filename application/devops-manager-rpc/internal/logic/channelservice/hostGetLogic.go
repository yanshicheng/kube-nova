package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostGetLogic {
	return &HostGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostGetLogic) HostGet(in *pb.GetByIdReq) (*pb.GetHostResp, error) {
	data, err := l.svcCtx.HostModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("主机查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetHostResp{Data: hostToPb(data)}, nil
}
