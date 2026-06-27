package portalprojectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectByPlatformLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectByPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectByPlatformLogic {
	return &GetProjectByPlatformLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProjectByPlatformLogic) GetProjectByPlatform(in *pb.GetProjectByPlatformReq) (*pb.GetProjectByPlatformResp, error) {
	if in.PlatformId == 0 {
		return nil, errorx.Msg("平台ID不能为空")
	}
	if platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.PlatformId); err != nil {
		l.Errorf("查询平台项目失败，平台不存在: %v", err)
		return nil, errorx.Msg("平台不存在")
	} else if platform.IsDeleted != 0 {
		return nil, errorx.Msg("平台不存在")
	}

	bindings, err := l.svcCtx.ProjectPlatformBindingModel.SearchNoPage(l.ctx, "id", true, "`platform_id` = ?", in.PlatformId)
	if err != nil {
		l.Errorf("查询平台项目绑定失败: %v", err)
		return nil, errorx.Msg("查询平台项目失败")
	}
	ids := make([]uint64, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.ProjectId)
	}
	projects, err := l.svcCtx.OnecProjectModel.BatchFindByIds(l.ctx, ids)
	if err != nil {
		l.Errorf("查询平台项目失败: %v", err)
		return nil, errorx.Msg("查询平台项目失败")
	}

	return &pb.GetProjectByPlatformResp{Data: projectsToPb(projects)}, nil
}
