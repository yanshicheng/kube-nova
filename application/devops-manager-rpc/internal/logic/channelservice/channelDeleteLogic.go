package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelDeleteLogic {
	return &ChannelDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelDeleteLogic) ChannelDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道删除失败: %v", err)
		return nil, err
	}
	if data.IsSystem {
		l.Errorf("系统渠道不能删除")
		return nil, errorx.Msg("系统渠道不能删除")
	}
	if data.ChannelType == tektonsync.EngineTypeTekton {
		if err := tektonsync.NewSyncer(l.svcCtx).DeleteAllStepsFromChannel(l.ctx, data, in.UpdatedBy); err != nil {
			l.Errorf("Tekton 渠道远端 Task 清理失败: %v", err)
			return nil, errorx.Msg("Tekton 渠道远端 Task 清理失败，请检查权限")
		}
	}
	if err := l.svcCtx.ChannelModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("渠道删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
