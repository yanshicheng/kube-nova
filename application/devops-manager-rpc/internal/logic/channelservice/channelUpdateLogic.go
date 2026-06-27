package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelUpdateLogic {
	return &ChannelUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelUpdateLogic) ChannelUpdate(in *pb.UpdateChannelReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("渠道更新失败: %v", err)
		return nil, err
	}
	if err := validateChannelScope(l.ctx, l.svcCtx, in.GroupId, in.ChannelType, in.CredentialId); err != nil {
		l.Errorf("渠道更新失败: %v", err)
		return nil, err
	}
	password := in.Password
	if password == maskedSecret {
		password = exist.Password
	}
	token := in.Token
	if token == maskedSecret {
		token = exist.Token
	}
	data := &model.DevopsChannel{
		ID:                 exist.ID,
		GroupID:            in.GroupId,
		Name:               in.Name,
		Code:               exist.Code,
		ChannelType:        in.ChannelType,
		Endpoint:           in.Endpoint,
		Description:        in.Description,
		GlobalCredentialID: in.GlobalCredentialId,
		CredentialID:       in.CredentialId,
		Config:             in.Config,
		Labels:             in.Labels,
		AuthType:           normalizeAuthType(in.AuthType),
		Username:           in.Username,
		Password:           password,
		Token:              token,
		InsecureSkipTLS:    in.InsecureSkipTls,
		Icon:               in.Icon,
		IconColor:          in.IconColor,
		Status:             in.Status,
		UpdatedBy:          in.UpdatedBy,
	}
	if err := tektonsync.ValidateChannelConfig(l.ctx, l.svcCtx, data); err != nil {
		l.Errorf("渠道更新失败: %v", err)
		return nil, err
	}

	if exist.ChannelType == tektonsync.EngineTypeTekton && data.ChannelType != tektonsync.EngineTypeTekton {
		if err := tektonsync.NewSyncer(l.svcCtx).DeleteAllStepsFromChannel(l.ctx, exist, in.UpdatedBy); err != nil {
			l.Errorf("Tekton 渠道类型变更前清理旧 Task 失败: %v", err)
			return nil, errorx.Msg("Tekton 渠道旧 Task 清理失败，请检查权限")
		}
	}

	if data.ChannelType == tektonsync.EngineTypeTekton {
		result := l.svcCtx.ChannelChecker.Test(l.ctx, data)
		if !result.Success {
			l.Errorf("Tekton 渠道连通性检测失败: %s", result.Message)
			return nil, errorx.Msg("Tekton 渠道连通性检测失败")
		}
		if err := l.svcCtx.ChannelModel.Update(l.ctx, data); err != nil {
			if model.IsDuplicateKey(err) {
				l.Errorf("渠道编码已存在")
				return nil, errorx.Msg("渠道编码已存在")
			}
			l.Errorf("渠道更新失败: %v", err)
			return nil, err
		}
		if err := l.svcCtx.ChannelModel.UpdateHealth(l.ctx, data.ID.Hex(), result.HealthStatus, result.Message, result.Metadata, in.UpdatedBy); err != nil {
			l.Errorf("渠道连通性状态更新失败: %v", err)
		}
		syncer := tektonsync.NewSyncer(l.svcCtx)
		if data.Status == 1 {
			if err := syncer.SyncAllStepsToChannel(l.ctx, data, in.UpdatedBy); err != nil {
				l.Errorf("Tekton 渠道同步步骤失败: %v", err)
				if cleanErr := syncer.DeleteAllStepsFromChannel(l.ctx, data, in.UpdatedBy); cleanErr != nil {
					l.Errorf("Tekton 渠道同步失败后清理新命名空间失败: %v", cleanErr)
				}
				l.restoreTektonChannel(exist, in.UpdatedBy)
				if exist.Status == 1 {
					if restoreErr := syncer.SyncAllStepsToChannel(l.ctx, exist, in.UpdatedBy); restoreErr != nil {
						l.Errorf("Tekton 渠道同步失败后恢复旧步骤失败: %v", restoreErr)
					}
				}
				return nil, errorx.Msg("Tekton 渠道同步步骤失败：" + devopstypes.TrimMessage(err.Error()))
			}
		}
		if data.Status != 1 || tektonChannelTaskNamespaceChanged(exist.Config, data.Config) {
			if err := syncer.DeleteAllStepsFromChannel(l.ctx, exist, in.UpdatedBy); err != nil {
				l.Errorf("Tekton 渠道旧 Task 命名空间清理失败: %v", err)
				if data.Status == 1 && tektonChannelTaskNamespaceChanged(exist.Config, data.Config) {
					if cleanErr := syncer.DeleteAllStepsFromChannel(l.ctx, data, in.UpdatedBy); cleanErr != nil {
						l.Errorf("Tekton 渠道新 Task 命名空间回滚清理失败: %v", cleanErr)
					}
				}
				l.restoreTektonChannel(exist, in.UpdatedBy)
				return nil, errorx.Msg("Tekton 渠道旧 Task 命名空间清理失败：" + devopstypes.TrimMessage(err.Error()))
			}
		}
		return &pb.EmptyResp{}, nil
	}

	if err := l.svcCtx.ChannelModel.Update(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("渠道编码已存在")
			return nil, errorx.Msg("渠道编码已存在")
		}
		l.Errorf("渠道更新失败: %v", err)
		return nil, err
	}
	result := l.svcCtx.ChannelChecker.Test(l.ctx, data)
	if err := l.svcCtx.ChannelModel.UpdateHealth(l.ctx, data.ID.Hex(), result.HealthStatus, result.Message, result.Metadata, in.UpdatedBy); err != nil {
		l.Errorf("渠道连通性状态更新失败: %v", err)
	}

	return &pb.EmptyResp{}, nil
}

func tektonChannelTaskNamespaceChanged(oldConfig, newConfig string) bool {
	oldCfg, oldErr := devopstekton.ParseChannelConfig(oldConfig)
	newCfg, newErr := devopstekton.ParseChannelConfig(newConfig)
	return oldErr == nil && newErr == nil && oldCfg.TaskNamespace != "" && oldCfg.TaskNamespace != newCfg.TaskNamespace
}

func (l *ChannelUpdateLogic) restoreTektonChannel(exist *model.DevopsChannel, operator string) {
	if exist == nil {
		return
	}
	exist.UpdatedBy = operator
	if err := l.svcCtx.ChannelModel.Update(l.ctx, exist); err != nil {
		l.Errorf("Tekton 渠道更新失败后恢复数据库失败: %v", err)
	}
}
