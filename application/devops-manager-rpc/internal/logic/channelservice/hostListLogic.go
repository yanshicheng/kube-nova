package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostListLogic {
	return &HostListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostListLogic) HostList(in *pb.ListHostReq) (*pb.ListHostResp, error) {
	list, total, err := l.svcCtx.HostModel.List(l.ctx, model.DevopsHostListFilter{
		Name:     in.Name,
		IP:       in.Ip,
		Status:   in.Status,
		Page:     in.Page,
		PageSize: in.PageSize,
	})
	if err != nil {
		l.Errorf("主机查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsHost, 0, len(list))
	for _, item := range list {
		items = append(items, hostToPb(item))
	}

	return &pb.ListHostResp{Data: items, Total: total}, nil
}
