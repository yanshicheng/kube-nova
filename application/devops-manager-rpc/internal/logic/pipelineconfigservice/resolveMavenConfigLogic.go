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

type ResolveMavenConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveMavenConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveMavenConfigLogic {
	return &ResolveMavenConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveMavenConfigLogic) ResolveMavenConfig(in *pb.ResolveMavenConfigReq) (*pb.ResolveMavenConfigResp, error) {
	if strings.TrimSpace(in.ProjectId) == "" {
		l.Errorf("解析 Maven 配置失败: 项目不能为空")
		return nil, errorx.Msg("项目不能为空")
	}
	if strings.TrimSpace(in.MavenConfigId) == "" {
		l.Errorf("解析 Maven 配置失败: Maven 配置不能为空")
		return nil, errorx.Msg("Maven 配置不能为空")
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析 Maven 配置失败: %v", err)
		return nil, err
	}
	config, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.MavenConfigId)
	if err == nil {
		if config.ProjectID != in.ProjectId {
			l.Errorf("Maven 配置不属于当前项目")
			return nil, errorx.Msg("Maven 配置不属于当前项目")
		}
		if config.TypeCode != model.DefaultMavenSettingsTypeCode {
			l.Errorf("Maven 配置类型不匹配")
			return nil, errorx.Msg("Maven 配置类型不匹配")
		}
		if config.Status != 1 {
			l.Errorf("Maven 配置已停用")
			return nil, errorx.Msg("Maven 配置已停用")
		}
		if strings.TrimSpace(config.Content) == "" {
			l.Errorf("Maven 配置内容为空")
			return nil, errorx.Msg("Maven 配置内容为空")
		}
		return &pb.ResolveMavenConfigResp{
			Content: config.Content,
			Name:    config.Name,
			Code:    config.Code,
		}, nil
	}
	if !errors.Is(err, model.ErrNotFound) && !errors.Is(err, model.ErrInvalidObjectID) {
		l.Errorf("查询 Maven 配置失败: %v", err)
		return nil, err
	}
	data, err := l.svcCtx.ProjectMavenModel.FindOne(l.ctx, in.MavenConfigId)
	if err != nil {
		l.Errorf("查询 Maven 配置失败: %v", err)
		return nil, err
	}
	if data.ProjectID != in.ProjectId {
		l.Errorf("Maven 配置不属于当前项目")
		return nil, errorx.Msg("Maven 配置不属于当前项目")
	}
	if data.Status != 1 {
		l.Errorf("Maven 配置已停用")
		return nil, errorx.Msg("Maven 配置已停用")
	}
	if strings.TrimSpace(data.Content) == "" {
		l.Errorf("Maven 配置内容为空")
		return nil, errorx.Msg("Maven 配置内容为空")
	}

	return &pb.ResolveMavenConfigResp{
		Content: data.Content,
		Name:    data.Name,
		Code:    data.Code,
	}, nil
}
