package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepStatusLogic {
	return &TektonStepStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepStatusLogic) TektonStepStatus(in *pb.UpdateStatusReq) (*pb.EmptyResp, error) {
	if in.Status != 0 && in.Status != 1 {
		l.Errorf("状态不支持")
		return nil, errorx.Msg("状态不支持")
	}
	step, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Tekton步骤更新状态失败: %v", err)
		return nil, err
	}
	if step.EngineType != tektonEngineType {
		l.Errorf("只能更新 Tekton 步骤状态")
		return nil, errorx.Msg("只能更新 Tekton 步骤状态")
	}
	step.Status = in.Status
	if in.Status == 1 {
		if err := tektonsync.NewSyncer(l.svcCtx).SyncStepToAllChannels(l.ctx, step, in.UpdatedBy); err != nil {
			l.Errorf("Tekton步骤启用后同步失败: %v", err)
			if cleanErr := tektonsync.NewSyncer(l.svcCtx).DeleteStepFromAllChannels(l.ctx, step, in.UpdatedBy); cleanErr != nil {
				l.Errorf("Tekton步骤启用失败后远端清理失败: %v", cleanErr)
			}
			return nil, errorx.Msg("Tekton 步骤同步失败：" + devopstypes.TrimMessage(err.Error()))
		}
	} else {
		if err := tektonsync.NewSyncer(l.svcCtx).DeleteStepFromAllChannels(l.ctx, step, in.UpdatedBy); err != nil {
			l.Errorf("Tekton步骤停用后同步清理失败: %v", err)
			return nil, errorx.Msg("Tekton 步骤同步清理失败：" + devopstypes.TrimMessage(err.Error()))
		}
	}
	if err := l.svcCtx.StepTemplateModel.UpdateStatus(l.ctx, in.Id, in.Status, in.UpdatedBy); err != nil {
		l.Errorf("Tekton步骤更新状态失败: %v", err)
		syncer := tektonsync.NewSyncer(l.svcCtx)
		if in.Status == 1 {
			if cleanErr := syncer.DeleteStepFromAllChannels(l.ctx, step, in.UpdatedBy); cleanErr != nil {
				l.Errorf("Tekton步骤状态更新失败后远端清理失败: %v", cleanErr)
			}
		} else {
			step.Status = 1
			if restoreErr := syncer.SyncStepToAllChannels(l.ctx, step, in.UpdatedBy); restoreErr != nil {
				l.Errorf("Tekton步骤状态更新失败后恢复远端资源失败: %v", restoreErr)
			}
		}
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
