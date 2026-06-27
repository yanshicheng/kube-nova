package executionservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineDeleteLogic {
	return &PipelineDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineDeleteLogic) PipelineDelete(in *pb.DeletePipelineReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.PipelineModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线删除失败: %v", err)
		return nil, err
	}
	if err := ensurePipelineAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线删除失败: %v", err)
		return nil, err
	}
	if data.EngineType == engineTekton && strings.TrimSpace(data.BuildChannelBindingID) != "" {
		if runtime, err := buildRuntime(l.ctx, l.svcCtx, data.ProjectID, data.SystemID, data.EnvironmentID, data.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles); err != nil {
			l.Errorf("删除 Tekton Pipeline 失败: pipelineId=%s err=%v", data.ID.Hex(), err)
			return nil, err
		} else if err := deleteTektonNativePrunerConfig(l.ctx, data, runtime); err != nil {
			l.Errorf("删除 Tekton Pruner ConfigMap 失败: pipelineId=%s err=%v", data.ID.Hex(), err)
			return nil, err
		} else if err := deleteTektonTrigger(l.ctx, data, runtime); err != nil {
			l.Errorf("删除 Tekton Trigger 失败: pipelineId=%s err=%v", data.ID.Hex(), err)
			return nil, err
		} else if err := deleteTektonPipeline(l.ctx, data, runtime); err != nil {
			l.Errorf("删除 Tekton Pipeline 失败: pipelineId=%s err=%v", data.ID.Hex(), err)
			return nil, err
		}
	} else if strings.TrimSpace(data.BuildChannelBindingID) != "" && strings.TrimSpace(data.JobFullName) != "" {
		if runtime, err := buildRuntime(l.ctx, l.svcCtx, data.ProjectID, data.SystemID, data.EnvironmentID, data.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles); err != nil {
			l.Errorf("停用 Jenkins Job 失败: pipelineId=%s err=%v", data.ID.Hex(), err)
		} else if manager := jenkinsManagerFromRuntime(runtime); manager != nil {
			if err := manager.DisableJob(l.ctx, data.JobFullName); err != nil {
				l.Errorf("停用 Jenkins Job 失败: pipelineId=%s jobFullName=%s err=%v", data.ID.Hex(), data.JobFullName, err)
			}
		}
	}
	if err := l.svcCtx.PipelineModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("流水线删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
