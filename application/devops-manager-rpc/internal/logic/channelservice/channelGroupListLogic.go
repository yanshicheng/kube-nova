package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGroupListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGroupListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGroupListLogic {
	return &ChannelGroupListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGroupListLogic) ChannelGroupList(in *pb.ListChannelGroupReq) (*pb.ListChannelGroupResp, error) {
	data, total, err := l.svcCtx.ChannelGroupModel.List(l.ctx, model.DevopsChannelGroupListFilter{
		Name:      in.Name,
		Code:      in.Code,
		Status:    in.Status,
		GroupType: in.GroupType,
		Page:      in.Page,
		PageSize:  in.PageSize,
	})
	if err != nil {
		l.Errorf("渠道分组查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsChannelGroup, 0, len(data))
	for _, item := range data {
		items = append(items, channelGroupToPb(item))
	}

	return &pb.ListChannelGroupResp{Data: items, Total: total}, nil
}
