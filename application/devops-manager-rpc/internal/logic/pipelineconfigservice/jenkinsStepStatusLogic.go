package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsStepStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsStepStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsStepStatusLogic {
	return &JenkinsStepStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsStepStatusLogic) JenkinsStepStatus(in *pb.UpdateStatusReq) (*pb.EmptyResp, error) {
	if in.Status != 0 && in.Status != 1 {
		l.Errorf("状态不支持")
		return nil, errorx.Msg("状态不支持")
	}
	if err := l.svcCtx.StepTemplateModel.UpdateStatus(l.ctx, in.Id, in.Status, in.UpdatedBy); err != nil {
		l.Errorf("Jenkins步骤更新状态失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
