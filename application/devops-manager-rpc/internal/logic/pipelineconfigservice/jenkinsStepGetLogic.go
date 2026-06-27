package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsStepGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsStepGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsStepGetLogic {
	return &JenkinsStepGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsStepGetLogic) JenkinsStepGet(in *pb.GetByIdReq) (*pb.GetStepTemplateResp, error) {
	data, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Jenkins步骤查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetStepTemplateResp{Data: stepTemplateToPb(l.ctx, l.svcCtx, data)}, nil
}
