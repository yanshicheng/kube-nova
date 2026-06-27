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

type SystemCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSystemCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemCreateLogic {
	return &SystemCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SystemCreateLogic) SystemCreate(in *pb.CreateSystemReq) (*pb.IdResp, error) {
	projectID := strings.TrimSpace(in.ProjectId)
	if err := validatePipelineCode(in.Code, "系统编码"); err != nil {
		l.Errorf("系统创建失败: %v", err)
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		l.Errorf("系统名称不能为空")
		return nil, errorx.Msg("系统名称不能为空")
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, projectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("系统创建失败: %v", err)
		return nil, err
	}
	if _, err := l.svcCtx.ProjectModel.FindOne(l.ctx, projectID); err != nil {
		l.Errorf("系统创建失败: %v", err)
		return nil, err
	}
	if _, err := l.svcCtx.SystemModel.FindOneByProjectCode(l.ctx, projectID, strings.TrimSpace(in.Code)); err == nil {
		l.Errorf("同项目下系统编码已存在")
		return nil, errorx.Msg("同项目下系统编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("系统创建失败: %v", err)
		return nil, err
	}

	data := &model.DevopsSystem{
		ProjectID:    projectID,
		Name:         strings.TrimSpace(in.Name),
		Code:         strings.TrimSpace(in.Code),
		Description:  in.Description,
		OwnerUserIDs: in.OwnerUserIds,
		Status:       in.Status,
		CreatedBy:    in.CreatedBy,
		UpdatedBy:    in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.SystemModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("同项目下系统编码已存在")
			return nil, errorx.Msg("同项目下系统编码已存在")
		}
		l.Errorf("系统创建失败: %v", err)
		return nil, err
	}
	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
