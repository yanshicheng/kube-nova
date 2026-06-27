package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserPlatformGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserPlatformGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPlatformGetLogic {
	return &UserPlatformGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserPlatformGet 获取用户可访问的平台列表
func (l *UserPlatformGetLogic) UserPlatformGet(in *pb.GetUserPlatformsReq) (*pb.GetUserPlatformsResp, error) {
	roles, _ := l.ctx.Value("roles").([]string)
	isSuperAdmin := false
	for _, role := range roles {
		if strings.ToUpper(role) == "SUPER_ADMIN" {
			isSuperAdmin = true
			break
		}
	}
	var pbPlatforms []*pb.SysPlatform

	if isSuperAdmin {
		platforms, err := l.listSuperAdminPlatforms(in.ProjectId)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("超管查询平台失败: projectId=%d, err=%v", in.ProjectId, err)
			return nil, errorx.Msg("查询平台列表失败")
		}
		for _, p := range platforms {
			pbPlatforms = append(pbPlatforms, l.convertModelToPb(p))
		}
	} else {
		userId, ok := l.ctx.Value("userId").(uint64)
		if !ok || userId == 0 {
			l.Errorf("获取用户平台列表失败：用户ID无效")
			return nil, errorx.Msg("用户ID无效")
		}

		var platformIds []uint64
		var err error
		if in.ProjectId > 0 {
			platformIds, err = l.svcCtx.ProjectMemberPlatformRole.ListPlatformIdsByUserAndProject(l.ctx, userId, in.ProjectId)
		} else {
			platformIds, err = l.svcCtx.ProjectMemberPlatformRole.ListPlatformIdsByUser(l.ctx, userId)
		}
		if err != nil {
			l.Errorf("按项目查询用户平台失败: userId=%d, projectId=%d, error=%v", userId, in.ProjectId, err)
			return nil, errorx.Msg("查询用户平台列表失败")
		}

		for _, platformId := range platformIds {
			platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, platformId)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					continue
				}
				l.Errorf("查询平台详情失败: id=%d, err=%v", platformId, err)
				return nil, errorx.Msg("查询平台详情失败")
			}

			if platform.IsEnable == 1 && platform.IsDeleted == 0 {
				pbPlatforms = append(pbPlatforms, l.convertModelToPb(platform))
			}
		}
	}

	l.Infof("获取用户平台列表成功: isSuperAdmin=%v, count=%d", isSuperAdmin, len(pbPlatforms))
	return &pb.GetUserPlatformsResp{
		Data: pbPlatforms,
	}, nil
}

func (l *UserPlatformGetLogic) listSuperAdminPlatforms(projectId uint64) ([]*model.SysPlatform, error) {
	if projectId == 0 {
		return l.svcCtx.SysPlatformModel.SearchNoPage(l.ctx, "sort", true, "`is_enable` = ? AND `is_deleted` = ?", 1, 0)
	}

	bindings, err := l.svcCtx.ProjectPlatformBindingModel.SearchNoPage(l.ctx, "id", true, "`project_id` = ?", projectId)
	if err != nil {
		return nil, err
	}

	platforms := make([]*model.SysPlatform, 0, len(bindings))
	for _, binding := range bindings {
		if binding.IsDeleted != 0 {
			continue
		}
		platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, binding.PlatformId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if platform.IsEnable == 1 && platform.IsDeleted == 0 {
			platforms = append(platforms, platform)
		}
	}

	return platforms, nil
}

// 辅助方法：统一转换模型
func (l *UserPlatformGetLogic) convertModelToPb(platform *model.SysPlatform) *pb.SysPlatform {
	return &pb.SysPlatform{
		Id:           platform.Id,
		PlatformCode: platform.PlatformCode,
		PlatformName: platform.PlatformName,
		PlatformDesc: platform.PlatformDesc,
		PlatformIcon: platform.PlatformIcon,
		Sort:         platform.Sort,
		IsEnable:     platform.IsEnable,
		IsDefault:    platform.IsDefault,
		CreateTime:   platform.CreateTime.Unix(),
		UpdateTime:   platform.UpdateTime.Unix(),
		CreateBy:     platform.CreateBy.String,
		UpdateBy:     platform.UpdateBy.String,
	}
}
