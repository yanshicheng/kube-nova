package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsStepDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsStepDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsStepDeleteLogic {
	return &JenkinsStepDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsStepDeleteLogic) JenkinsStepDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	total, err := l.svcCtx.PipelineTemplateModel.CountByStepID(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Jenkins步骤删除失败: %v", err)
		return nil, err
	}
	if total > 0 {
		l.Errorf("步骤已被流水线模板引用，不能删除")
		return nil, errorx.Msg("步骤已被流水线模板引用，不能删除")
	}
	if err := l.svcCtx.StepTemplateModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("Jenkins步骤删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
