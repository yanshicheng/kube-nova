package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepDeleteLogic {
	return &TektonStepDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepDeleteLogic) TektonStepDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	step, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Tekton步骤删除失败: %v", err)
		return nil, err
	}
	if step.EngineType != tektonEngineType {
		l.Errorf("只能删除 Tekton 步骤")
		return nil, errorx.Msg("只能删除 Tekton 步骤")
	}
	total, err := l.svcCtx.PipelineTemplateModel.CountByStepID(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Tekton步骤删除失败: %v", err)
		return nil, err
	}
	if total > 0 {
		l.Errorf("步骤已被流水线模板引用，不能删除")
		return nil, errorx.Msg("步骤已被流水线模板引用，不能删除")
	}
	if err := tektonsync.NewSyncer(l.svcCtx).DeleteStepFromAllChannels(l.ctx, step, in.UpdatedBy); err != nil {
		l.Errorf("Tekton步骤删除前同步清理失败: %v", err)
		return nil, errorx.Msg("Tekton 步骤远端清理失败，请检查 Tekton 实例配置")
	}
	if err := l.svcCtx.StepTemplateModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("Tekton步骤删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.StepSyncModel.MarkDeletedByStep(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("Tekton步骤同步记录标记删除失败: %v", err)
	}

	return &pb.EmptyResp{}, nil
}
