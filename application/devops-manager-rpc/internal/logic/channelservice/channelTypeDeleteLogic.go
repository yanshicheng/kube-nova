package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTypeDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTypeDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTypeDeleteLogic {
	return &ChannelTypeDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTypeDeleteLogic) ChannelTypeDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.ChannelTypeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道类型删除失败: %v", err)
		return nil, err
	}
	if data.IsSystem {
		l.Errorf("系统渠道类型不能删除")
		return nil, errorx.Msg("系统渠道类型不能删除")
	}
	if err := l.svcCtx.ChannelTypeModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("渠道类型删除失败: %v", err)
		return nil, err
	}
	if err := syncGroupAllowedChannelType(l.ctx, l.svcCtx, data.GroupCode, data.Code, in.UpdatedBy, false); err != nil {
		l.Errorf("同步渠道分组允许类型失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
