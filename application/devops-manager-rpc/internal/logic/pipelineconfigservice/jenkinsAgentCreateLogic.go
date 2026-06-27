package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsAgentCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsAgentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsAgentCreateLogic {
	return &JenkinsAgentCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsAgentCreateLogic) JenkinsAgentCreate(in *pb.CreateJenkinsAgentReq) (*pb.IdResp, error) {
	if err := ensureJenkinsChannel(l.ctx, l.svcCtx, in.ChannelId); err != nil {
		l.Errorf("JenkinsAgent创建失败: %v", err)
		return nil, err
	}
	data, err := normalizeJenkinsAgentInput(
		in.ChannelId,
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
		l.Errorf("JenkinsAgent创建失败: %v", err)
		return nil, err
	}
	data.Status = in.Status
	data.CreatedBy = in.CreatedBy
	data.UpdatedBy = in.CreatedBy
	if data.AgentType == "dynamic" && len(data.Containers) == 0 {
		data.Containers = model.DefaultJenkinsAgentContainers()
	}
	if err := l.svcCtx.JenkinsAgentModel.Insert(l.ctx, data); err != nil {
		l.Errorf("JenkinsAgent创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
