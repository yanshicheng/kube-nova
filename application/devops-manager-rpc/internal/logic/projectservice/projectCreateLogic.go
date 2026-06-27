package projectservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	portalpb "github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectCreateLogic {
	return &ProjectCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectCreateLogic) ProjectCreate(in *pb.CreateProjectReq) (*pb.IdResp, error) {
	// 1. 先调用 portal-rpc 创建统一项目
	portalResp, err := l.svcCtx.PortalRpc.CreateProject(l.ctx, &portalpb.PortalCreateProjectReq{
		Name:        in.Name,
		IsSystem:    0,
		Description: in.Description,
		CreatedBy:   in.CreatedBy,
	})
	if err != nil {
		l.Errorf("调用 portal 创建项目失败: %v", err)
		return nil, errorx.Msg("创建项目失败")
	}

	// 2. 创建 DevOps 扩展记录
	defaultChannelID, buildChannels, err := prepareProjectBuildChannels(l.ctx, l.svcCtx, in.BuildChannelIds, in.DefaultEngineChannelId)
	if err != nil {
		l.Errorf("准备构建渠道失败: %v", err)
		return nil, err
	}

	data := &model.DevopsProject{
		Name:                   in.Name,
		Code:                   generateProjectCode(),
		PortalProjectUuid:      portalResp.Uuid,
		Description:            in.Description,
		PipelineEngineType:     defaultBuildChannelType(buildChannels, defaultChannelID),
		DefaultEngineChannelID: defaultChannelID,
		Status:                 in.Status,
		ExtraConfig:            in.ExtraConfig,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}

	if err := l.svcCtx.ProjectModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("项目编码已存在")
			return nil, errorx.Msg("项目编码已存在")
		}
		l.Errorf("DevOps 项目创建失败: %v", err)
		return nil, err
	}

	if err := replaceProjectBuildChannels(l.ctx, l.svcCtx, data.ID.Hex(), buildChannels, defaultChannelID, true, "", "", in.CreatedBy); err != nil {
		_ = l.svcCtx.ProjectModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy)
		l.Errorf("创建构建渠道绑定失败: %v", err)
		return nil, err
	}

	l.Infof("DevOps 项目创建成功，ID: %s, portalProjectUuid: %s", data.ID.Hex(), portalResp.Uuid)
	return &pb.IdResp{Id: data.ID.Hex()}, nil
}

func generateProjectCode() string {
	return fmt.Sprintf("proj-%s-%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
}
