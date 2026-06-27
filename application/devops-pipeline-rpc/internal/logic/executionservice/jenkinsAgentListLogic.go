package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsAgentListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsAgentListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsAgentListLogic {
	return &JenkinsAgentListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsAgentListLogic) JenkinsAgentList(in *pb.JenkinsAgentListReq) (*pb.JenkinsAgentListResp, error) {
	runtime, err := buildRuntime(l.ctx, l.svcCtx, in.ProjectId, in.SystemId, in.EnvironmentId, in.BuildChannelBindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("JenkinsAgent查询列表失败: %v", err)
		return nil, err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	agents, err := manager.ListAgents(l.ctx)
	if err != nil {
		l.Errorf("JenkinsAgent查询列表失败: %v", err)
		return nil, err
	}
	data := make([]*pb.JenkinsAgentNode, 0, len(agents))
	for _, item := range agents {
		data = append(data, &pb.JenkinsAgentNode{
			Name:               item.Name,
			DisplayName:        item.DisplayName,
			Label:              item.Label,
			Offline:            item.Offline,
			TemporarilyOffline: item.TemporarilyOffline,
			Labels:             item.Labels,
		})
	}

	return &pb.JenkinsAgentListResp{Data: data}, nil
}
