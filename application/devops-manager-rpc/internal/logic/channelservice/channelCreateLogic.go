package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelCreateLogic {
	return &ChannelCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelCreateLogic) ChannelCreate(in *pb.CreateChannelReq) (*pb.IdResp, error) {
	if err := validateChannelScope(l.ctx, l.svcCtx, in.GroupId, in.ChannelType, in.CredentialId); err != nil {
		l.Errorf("渠道创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsChannel{
		GroupID:            in.GroupId,
		Name:               in.Name,
		Code:               in.Code,
		ChannelType:        in.ChannelType,
		Endpoint:           in.Endpoint,
		Description:        in.Description,
		GlobalCredentialID: in.GlobalCredentialId,
		CredentialID:       in.CredentialId,
		Config:             in.Config,
		Labels:             in.Labels,
		AuthType:           normalizeAuthType(in.AuthType),
		Username:           in.Username,
		Password:           in.Password,
		Token:              in.Token,
		InsecureSkipTLS:    in.InsecureSkipTls,
		Icon:               in.Icon,
		IconColor:          in.IconColor,
		Status:             in.Status,
		CreatedBy:          in.CreatedBy,
		UpdatedBy:          in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := tektonsync.ValidateChannelConfig(l.ctx, l.svcCtx, data); err != nil {
		l.Errorf("渠道创建失败: %v", err)
		return nil, err
	}

	if err := l.svcCtx.ChannelModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("渠道编码已存在")
			return nil, errorx.Msg("渠道编码已存在")
		}
		l.Errorf("渠道创建失败: %v", err)
		return nil, err
	}
	result := l.svcCtx.ChannelChecker.Test(l.ctx, data)
	if err := l.svcCtx.ChannelModel.UpdateHealth(l.ctx, data.ID.Hex(), result.HealthStatus, result.Message, result.Metadata, in.CreatedBy); err != nil {
		l.Errorf("渠道连通性状态更新失败: %v", err)
	}
	if data.ChannelType == tektonsync.EngineTypeTekton && !result.Success {
		l.Errorf("Tekton 渠道连通性检测失败: %s", result.Message)
		if deleteErr := l.svcCtx.ChannelModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy); deleteErr != nil {
			l.Errorf("Tekton 渠道连通性失败后回滚失败: %v", deleteErr)
		}
		return nil, errorx.Msg("Tekton 渠道连通性检测失败")
	}
	if data.ChannelType == tektonsync.EngineTypeTekton && result.Success {
		if err := tektonsync.NewSyncer(l.svcCtx).SyncAllStepsToChannel(l.ctx, data, in.CreatedBy); err != nil {
			l.Errorf("Tekton 渠道已创建但步骤同步失败: %v", err)
			if cleanErr := tektonsync.NewSyncer(l.svcCtx).DeleteAllStepsFromChannel(l.ctx, data, in.CreatedBy); cleanErr != nil {
				l.Errorf("Tekton 渠道步骤同步失败后远端清理失败: %v", cleanErr)
			}
			if deleteErr := l.svcCtx.ChannelModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy); deleteErr != nil {
				l.Errorf("Tekton 渠道步骤同步失败后回滚失败: %v", deleteErr)
			}
			return nil, errorx.Msg("Tekton 渠道步骤同步失败：" + devopstypes.TrimMessage(err.Error()))
		}
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
