package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

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

func (l *JenkinsAgentListLogic) JenkinsAgentList(in *pb.ListJenkinsAgentReq) (*pb.ListJenkinsAgentResp, error) {
	channelID, err := resolveJenkinsAgentChannelID(l.ctx, l.svcCtx, in.ChannelId, in.BuildChannelBindingId)
	if err != nil {
		l.Errorf("JenkinsAgent查询列表失败: %v", err)
		return nil, err
	}
	items, total, err := l.svcCtx.JenkinsAgentModel.List(l.ctx, model.DevopsJenkinsAgentListFilter{
		ChannelID: channelID,
		Name:      in.Name,
		AgentType: in.AgentType,
		Status:    in.Status,
		Page:      in.Page,
		PageSize:  in.PageSize,
	})
	if err != nil {
		l.Errorf("JenkinsAgent查询列表失败: %v", err)
		return nil, err
	}
	data := make([]*pb.DevopsJenkinsAgent, 0, len(items))
	for _, item := range items {
		data = append(data, jenkinsAgentToPb(l.ctx, l.svcCtx, item))
	}

	return &pb.ListJenkinsAgentResp{Data: data, Total: total}, nil
}
