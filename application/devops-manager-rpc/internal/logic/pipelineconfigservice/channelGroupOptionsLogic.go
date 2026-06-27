package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelGroupOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelGroupOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelGroupOptionsLogic {
	return &ChannelGroupOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelGroupOptionsLogic) ChannelGroupOptions(in *pb.EmptyResp) (*pb.ChannelGroupOptionsResp, error) {
	data, _, err := l.svcCtx.ChannelGroupModel.List(l.ctx, model.DevopsChannelGroupListFilter{
		Page:     1,
		PageSize: 200,
		Status:   1,
	})
	if err != nil {
		l.Errorf("渠道分组选项失败: %v", err)
		return nil, err
	}
	items := make([]*pb.ChannelGroupOption, 0, len(data))
	for _, item := range data {
		items = append(items, &pb.ChannelGroupOption{Code: item.Code, Name: item.Name})
	}

	return &pb.ChannelGroupOptionsResp{Data: items}, nil
}
