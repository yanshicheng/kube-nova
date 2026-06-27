package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsAgentUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsAgentUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsAgentUpdateLogic {
	return &JenkinsAgentUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsAgentUpdateLogic) JenkinsAgentUpdate(in *pb.UpdateJenkinsAgentReq) (*pb.EmptyResp, error) {
	old, err := l.svcCtx.JenkinsAgentModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("JenkinsAgent更新失败: %v", err)
		return nil, err
	}
	data, err := normalizeJenkinsAgentInput(
		old.ChannelID,
		in.Name,
		in.Code,
		in.AgentType,
		in.MatchMode,
		in.MatchValue,
		in.Cloud,
		in.PodYaml,
		jenkinsAgentContainersFromPb(in.Containers),
	)
	if err != nil {
		l.Errorf("JenkinsAgent更新失败: %v", err)
		return nil, err
	}
	data.ID = old.ID
	data.Status = in.Status
	data.UpdatedBy = in.UpdatedBy
	if data.AgentType == "dynamic" && len(data.Containers) == 0 {
		data.Containers = model.DefaultJenkinsAgentContainers()
	}
	if err := l.svcCtx.JenkinsAgentModel.Update(l.ctx, data); err != nil {
		l.Errorf("JenkinsAgent更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
