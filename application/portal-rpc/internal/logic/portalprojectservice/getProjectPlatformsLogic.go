package portalprojectservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectPlatformsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectPlatformsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectPlatformsLogic {
	return &GetProjectPlatformsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProjectPlatformsLogic) GetProjectPlatforms(in *pb.GetProjectPlatformsReq) (*pb.GetProjectPlatformsResp, error) {
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}
	if _, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId); err != nil {
		l.Errorf("查询项目平台失败，项目不存在: %v", err)
		return nil, errorx.Msg("项目不存在")
	}

	bindings, err := l.svcCtx.ProjectPlatformBindingModel.SearchNoPage(l.ctx, "id", true, "`project_id` = ?", in.ProjectId)
	if err != nil {
		l.Errorf("查询项目平台绑定失败: %v", err)
		return nil, errorx.Msg("查询项目平台失败")
	}

	data := make([]*pb.ProjectPlatformBinding, 0, len(bindings))
	for _, binding := range bindings {
		platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, binding.PlatformId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("查询平台详情失败: %v", err)
			return nil, errorx.Msg("查询平台详情失败")
		}
		if platform.IsDeleted != 0 {
			continue
		}
		data = append(data, &pb.ProjectPlatformBinding{
			Id:           binding.Id,
			ProjectId:    binding.ProjectId,
			PlatformId:   binding.PlatformId,
			PlatformCode: platform.PlatformCode,
			PlatformName: platform.PlatformName,
			CreatedBy:    binding.CreatedBy,
			CreatedAt:    binding.CreatedAt.Unix(),
		})
	}

	return &pb.GetProjectPlatformsResp{Data: data}, nil
}
