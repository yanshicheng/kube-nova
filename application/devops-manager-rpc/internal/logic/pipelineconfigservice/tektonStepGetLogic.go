package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepGetLogic {
	return &TektonStepGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepGetLogic) TektonStepGet(in *pb.GetByIdReq) (*pb.GetStepTemplateResp, error) {
	data, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Tekton步骤查询详情失败: %v", err)
		return nil, err
	}
	if data.EngineType != tektonEngineType {
		l.Errorf("只能查询 Tekton 步骤")
		return nil, errorx.Msg("只能查询 Tekton 步骤")
	}

	return &pb.GetStepTemplateResp{Data: stepTemplateToPb(l.ctx, l.svcCtx, data)}, nil
}
