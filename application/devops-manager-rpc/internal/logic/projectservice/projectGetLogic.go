package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	portalprojectservice "github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalprojectservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectGetLogic {
	return &ProjectGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectGetLogic) ProjectGet(in *pb.GetByIdReq) (*pb.GetProjectResp, error) {
	data, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目查询详情失败: %v", err)
		return nil, err
	}
	if data == nil || strings.TrimSpace(data.PortalProjectUuid) == "" {
		return nil, errorx.Msg("无权访问当前 DevOps 项目")
	}
	if !isSuperAdminRole(in.CurrentRoles) {
		if in.CurrentUserId == 0 {
			return nil, errorx.Msg("无权访问当前 DevOps 项目")
		}
		memberIDs, err := l.svcCtx.ProjectMemberModel.ListProjectIDsByUser(l.ctx, in.CurrentUserId)
		if err != nil {
			l.Errorf("校验项目详情权限失败: %v", err)
			return nil, err
		}
		allowed := false
		for _, id := range memberIDs {
			if id == in.Id {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errorx.Msg("无权访问当前 DevOps 项目")
		}
		platformID, _ := l.ctx.Value("platformId").(uint64)
		if platformID == 0 {
			return nil, errorx.Msg("无权访问当前 DevOps 项目")
		}
		portalResp, err := l.svcCtx.PortalRpc.ListProjects(l.ctx, &portalprojectservice.PortalListProjectsReq{
			Page:       1,
			PageSize:   1,
			UserId:     in.CurrentUserId,
			PlatformId: platformID,
		})
		if err != nil {
			l.Errorf("校验项目详情门户授权失败: %v", err)
			return nil, err
		}
		portalAllowed := false
		for _, item := range portalResp.Data {
			if item != nil && item.Uuid == data.PortalProjectUuid {
				portalAllowed = true
				break
			}
		}
		if !portalAllowed {
			return nil, errorx.Msg("无权访问当前 DevOps 项目")
		}
	}

	return &pb.GetProjectResp{Data: projectToPb(data)}, nil
}
