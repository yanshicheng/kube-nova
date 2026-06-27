package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineEnvironmentUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineEnvironmentUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineEnvironmentUpdateLogic {
	return &PipelineEnvironmentUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineEnvironmentUpdateLogic) PipelineEnvironmentUpdate(in *pb.UpdatePipelineEnvironmentReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.PipelineEnvModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线环境更新失败: %v", err)
		return nil, err
	}
	if err := ensureEnvironmentAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线环境更新失败: %v", err)
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		l.Errorf("环境名称不能为空")
		return nil, errorx.Msg("环境名称不能为空")
	}
	data.Name = strings.TrimSpace(in.Name)
	data.Description = in.Description
	data.Icon = in.Icon
	data.IconColor = in.IconColor
	data.SortOrder = in.SortOrder
	data.Status = in.Status
	data.UpdatedBy = in.UpdatedBy
	if err := l.svcCtx.PipelineEnvModel.Update(l.ctx, data); err != nil {
		l.Errorf("流水线环境更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
