package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunArtifactListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunArtifactListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunArtifactListLogic {
	return &PipelineRunArtifactListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunArtifactListLogic) PipelineRunArtifactList(in *pb.PipelineRunArtifactListReq) (*pb.PipelineRunArtifactListResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOne(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线运行产物查询列表失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线运行产物查询列表失败: %v", err)
		return nil, err
	}
	items, err := l.svcCtx.ArtifactModel.ListByRun(l.ctx, run.ID.Hex())
	if err != nil {
		l.Errorf("流水线运行产物查询列表失败: %v", err)
		return nil, err
	}
	resp := &pb.PipelineRunArtifactListResp{Data: make([]*pb.PipelineRunArtifact, 0, len(items))}
	for _, item := range items {
		out := runArtifactToPb(item)
		if item.Status == "success" && item.ObjectKey != "" {
			if urlResp, err := l.svcCtx.PipelineConfigRpc.PipelineArtifactUrl(l.ctx, &pipelineconfigservice.PipelineArtifactUrlReq{ObjectKey: item.ObjectKey}); err == nil {
				out.Url = urlResp.Url
			}
		}
		resp.Data = append(resp.Data, out)
	}

	return resp, nil
}

func runArtifactToPb(in *model.DevopsPipelineRunArtifact) *pb.PipelineRunArtifact {
	if in == nil {
		return nil
	}
	return &pb.PipelineRunArtifact{
		Id:           in.ID.Hex(),
		RunId:        in.RunID,
		StageId:      in.StageID,
		StepId:       in.StepID,
		Name:         in.Name,
		Type:         in.Type,
		Bucket:       in.Bucket,
		ObjectKey:    in.ObjectKey,
		Size:         in.Size,
		ContentType:  in.ContentType,
		Status:       in.Status,
		ErrorMessage: in.ErrorMessage,
		CreatedAt:    formatTime(in.CreateAt),
	}
}
