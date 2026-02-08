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

// UserPlatformGet 获取用户绑定的平台列表
func (l *UserPlatformGetLogic) UserPlatformGet(in *pb.GetUserPlatformsReq) (*pb.GetUserPlatformsResp, error) {
	// 1. 从 context 获取角色列表并判断是否为 SUPER_ADMIN super_admin
	roles, _ := l.ctx.Value("roles").([]string)
	isSuperAdmin := false
	for _, role := range roles {
		if strings.ToUpper(role) == "SUPER_ADMIN" {
			isSuperAdmin = true
			break
		}
	}
	var pbPlatforms []*pb.SysPlatform

	// 2. 如果是超级管理员，直接获取所有正常状态的平台
	if isSuperAdmin {
		platforms, err := l.svcCtx.SysPlatformModel.SearchNoPage(l.ctx, "sort", true, "`is_enable` = ? AND `is_deleted` = ?", 1, 0)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("超管查询全量平台失败: %v", err)
			return nil, errorx.Msg("查询平台列表失败")
		}
		for _, p := range platforms {
			pbPlatforms = append(pbPlatforms, l.convertModelToPb(p))
		}
	} else {
		// 3. 普通用户逻辑：按 userId 过滤绑定关系
		userId, ok := l.ctx.Value("userId").(uint64)
		if !ok || userId == 0 {
			l.Errorf("获取用户平台列表失败：用户ID无效")
			return nil, errorx.Msg("用户ID无效")
		}

		// 查询关联表
		userPlatforms, err := l.svcCtx.SysUserPlatformModel.SearchNoPage(l.ctx, "id", true, "`user_id` = ? AND `is_enable` = ? AND `status` = ?", userId, 1, 1)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return &pb.GetUserPlatformsResp{Data: []*pb.SysPlatform{}}, nil
			}
			l.Errorf("查询用户平台绑定失败: userId=%d, error=%v", userId, err)
			return nil, errorx.Msg("查询用户平台绑定失败")
		}

		// 遍历关联关系查询详情
		for _, up := range userPlatforms {
			platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, up.PlatformId)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					continue
				}
				l.Errorf("查询平台详情失败: id=%d, err=%v", up.PlatformId, err)
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
