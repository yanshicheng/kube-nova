package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTypeListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTypeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTypeListLogic {
	return &ChannelTypeListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTypeListLogic) ChannelTypeList(in *pb.ListChannelTypeReq) (*pb.ListChannelTypeResp, error) {
	list, total, err := l.svcCtx.ChannelTypeModel.List(l.ctx, model.DevopsChannelTypeListFilter{
		Name:      in.Name,
		Code:      in.Code,
		GroupCode: in.GroupCode,
		Status:    in.Status,
		Page:      in.Page,
		PageSize:  in.PageSize,
	})
	if err != nil {
		l.Errorf("渠道类型查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsChannelType, 0, len(list))
	for _, item := range list {
		items = append(items, channelTypeToPb(item))
	}

	return &pb.ListChannelTypeResp{Data: items, Total: total}, nil
}
