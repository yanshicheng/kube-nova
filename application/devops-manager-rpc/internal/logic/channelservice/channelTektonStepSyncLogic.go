package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTektonStepSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTektonStepSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTektonStepSyncLogic {
	return &ChannelTektonStepSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTektonStepSyncLogic) ChannelTektonStepSync(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Tekton 渠道步骤同步失败: %v", err)
		return nil, err
	}
	if channel.ChannelType != tektonsync.EngineTypeTekton {
		l.Errorf("只能同步 Tekton 渠道步骤")
		return nil, errorx.Msg("只能同步 Tekton 渠道步骤")
	}
	if channel.Status != 1 {
		l.Errorf("Tekton 渠道已停用")
		return nil, errorx.Msg("Tekton 渠道已停用")
	}
	if err := tektonsync.NewSyncer(l.svcCtx).SyncAllStepsToChannel(l.ctx, channel, in.UpdatedBy); err != nil {
		l.Errorf("Tekton 渠道步骤同步失败: %v", err)
		_ = l.svcCtx.ChannelModel.UpdateHealth(l.ctx, channel.ID.Hex(), "unhealthy", "Tekton 步骤同步失败："+err.Error(), channel.Metadata, in.UpdatedBy)
		return nil, errorx.Msg("Tekton 步骤同步失败：" + devopstypes.TrimMessage(err.Error()))
	}
	if channel.HealthStatus != "healthy" {
		_ = l.svcCtx.ChannelModel.UpdateHealth(l.ctx, channel.ID.Hex(), "healthy", "Tekton 步骤同步完成", channel.Metadata, in.UpdatedBy)
	}

	return &pb.EmptyResp{}, nil
}
