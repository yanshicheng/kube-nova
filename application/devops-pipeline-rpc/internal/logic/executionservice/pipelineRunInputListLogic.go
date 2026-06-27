package executionservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunInputListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunInputListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunInputListLogic {
	return &PipelineRunInputListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunInputListLogic) PipelineRunInputList(in *pb.PipelineRunInputListReq) (*pb.PipelineRunInputListResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOneForInput(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线人工确认查询失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线人工确认查询失败: %v", err)
		return nil, err
	}
	runtime, err := buildRuntimeCached(l.ctx, l.svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线人工确认查询失败: %v", err)
		return nil, err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	ready, err := ensureRunBuildNumber(l.ctx, l.svcCtx, run, manager, "")
	if err != nil {
		l.Errorf("流水线人工确认查询失败: %v", err)
		return nil, err
	}
	if !ready {
		return &pb.PipelineRunInputListResp{Data: []*pb.PipelineRunInput{}}, nil
	}
	items, err := manager.PendingInputs(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
	if err != nil {
		l.Errorf("流水线人工确认查询失败: %v", err)
		return nil, err
	}
	return &pb.PipelineRunInputListResp{Data: jenkinsInputsToPb(items)}, nil
}

func jenkinsInputsToPb(items []jenkins.InputAction) []*pb.PipelineRunInput {
	result := make([]*pb.PipelineRunInput, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.PipelineRunInput{
			Id:      firstNonEmptyString(item.ID, item.Name),
			Name:    item.Name,
			Message: item.Message,
			Params:  jenkinsInputParamsToPb(item.Parameters),
		})
	}
	return result
}

func jenkinsInputParamsToPb(items []jenkins.InputParameter) []*pb.PipelineRunInputParam {
	result := make([]*pb.PipelineRunInputParam, 0, len(items))
	for _, item := range items {
		options := make([]*pb.StepParamOption, 0, len(item.Choices))
		for _, choice := range item.Choices {
			choice = strings.TrimSpace(choice)
			if choice == "" {
				continue
			}
			options = append(options, &pb.StepParamOption{Label: choice, Value: choice})
		}
		result = append(result, &pb.PipelineRunInputParam{
			Name:         item.Name,
			Type:         normalizeInputParamType(item.Type, len(options) > 0),
			Description:  item.Description,
			DefaultValue: item.DefaultValue,
			Options:      options,
		})
	}
	return result
}

func normalizeInputParamType(value string, hasChoices bool) string {
	if hasChoices {
		return "choice"
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "booleanparameterdefinition", "boolean":
		return "boolean"
	case "passwordparameterdefinition", "password":
		return "password"
	case "textparameterdefinition", "text":
		return "text"
	case "stringparameterdefinition", "string":
		return "string"
	default:
		return "string"
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
