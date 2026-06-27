package portalprojectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	devopspb "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProjectLogic {
	return &CreateProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateProjectLogic) CreateProject(in *pb.PortalCreateProjectReq) (*pb.PortalCreateProjectResp, error) {
	if in.Name == "" {
		return nil, errorx.Msg("项目名称不能为空")
	}

	project, err := l.svcCtx.OnecProjectModel.InsertWithUuid(l.ctx, in.Name, in.IsSystem, in.Description, in.CreatedBy)
	if err != nil {
		l.Errorf("创建项目失败: %v", err)
		return nil, errorx.Msg("创建项目失败")
	}

	l.Infof("创建项目成功，ID: %d, UUID: %s, Name: %s", project.Id, project.Uuid, project.Name)

	// 同步到 DevOps
	if l.svcCtx.DevopsManagerRpc != nil {
		syncReq := &devopspb.DevopsSyncProjectInfoReq{
			PortalProjectUuid: project.Uuid,
			Name:              project.Name,
			Description:       in.Description,
		}
		_, syncErr := l.svcCtx.DevopsManagerRpc.SyncProjectInfo(l.ctx, syncReq)
		if syncErr != nil {
			l.Errorf("同步项目信息到 DevOps 失败: %v", syncErr)
		} else {
			l.Infof("同步项目到 DevOps 成功，UUID: %s", project.Uuid)
		}
	}

	return &pb.PortalCreateProjectResp{
		Id:   project.Id,
		Uuid: project.Uuid,
	}, nil
}
