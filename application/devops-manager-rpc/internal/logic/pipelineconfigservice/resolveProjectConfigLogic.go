package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveProjectConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveProjectConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveProjectConfigLogic {
	return &ResolveProjectConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveProjectConfigLogic) ResolveProjectConfig(in *pb.ResolveProjectConfigReq) (*pb.ResolveProjectConfigResp, error) {
	if strings.TrimSpace(in.ProjectId) == "" {
		l.Errorf("解析项目配置失败: 项目不能为空")
		return nil, errorx.Msg("项目不能为空")
	}
	if strings.TrimSpace(in.ConfigId) == "" {
		l.Errorf("解析项目配置失败: 配置不能为空")
		return nil, errorx.Msg("配置不能为空")
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析项目配置失败: %v", err)
		return nil, err
	}
	data, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		l.Errorf("查询项目配置失败: %v", err)
		return nil, err
	}
	if data.ProjectID != in.ProjectId {
		l.Errorf("项目配置不属于当前项目")
		return nil, errorx.Msg("项目配置不属于当前项目")
	}
	if strings.TrimSpace(in.TypeId) != "" && data.TypeID != strings.TrimSpace(in.TypeId) {
		l.Errorf("项目配置类型不匹配")
		return nil, errorx.Msg("项目配置类型不匹配")
	}
	if strings.TrimSpace(in.TypeCode) != "" && data.TypeCode != strings.TrimSpace(in.TypeCode) {
		l.Errorf("项目配置类型不匹配")
		return nil, errorx.Msg("项目配置类型不匹配")
	}
	if data.Status != 1 {
		l.Errorf("项目配置已停用")
		return nil, errorx.Msg("项目配置已停用")
	}
	if strings.TrimSpace(data.Content) == "" {
		l.Errorf("项目配置内容为空")
		return nil, errorx.Msg("项目配置内容为空")
	}

	return &pb.ResolveProjectConfigResp{
		Content:  data.Content,
		Name:     data.Name,
		Code:     data.Code,
		TypeCode: data.TypeCode,
	}, nil
}
