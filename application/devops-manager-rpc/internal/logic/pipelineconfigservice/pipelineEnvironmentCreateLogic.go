package pipelineconfigservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineEnvironmentCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineEnvironmentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineEnvironmentCreateLogic {
	return &PipelineEnvironmentCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineEnvironmentCreateLogic) PipelineEnvironmentCreate(in *pb.CreatePipelineEnvironmentReq) (*pb.IdResp, error) {
	if err := validatePipelineCode(in.Code, "环境编码"); err != nil {
		l.Errorf("流水线环境创建失败: %v", err)
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		l.Errorf("环境名称不能为空")
		return nil, errorx.Msg("环境名称不能为空")
	}
	if _, err := l.svcCtx.PipelineEnvModel.FindOneByCode(l.ctx, strings.TrimSpace(in.Code)); err == nil {
		l.Errorf("环境编码已存在")
		return nil, errorx.Msg("环境编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("流水线环境创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsPipelineEnvironment{
		Name:        strings.TrimSpace(in.Name),
		Code:        strings.TrimSpace(in.Code),
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.PipelineEnvModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("环境编码已存在")
			return nil, errorx.Msg("环境编码已存在")
		}
		l.Errorf("流水线环境创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
