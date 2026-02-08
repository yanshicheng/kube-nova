// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

const SuperAdminRole = "SUPER_ADMIN"

type GetUserPlatformsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserPlatformsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserPlatformsLogic {
	return &GetUserPlatformsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserPlatformsLogic) GetUserPlatforms() (resp []types.SysPlatform, err error) {
	// 获取上下文中的用户ID
	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		l.Errorf("获取用户平台列表失败：用户ID无效")
		return nil, fmt.Errorf("用户ID无效")
	}

	// 获取上下文中的用户角色
	roles, ok := l.ctx.Value("roles").([]string)
	if !ok {
		l.Errorf("获取用户平台列表失败：用户角色无效")
		return nil, fmt.Errorf("用户角色无效")
	}

	// 检查是否是超级管理员
	isSuperAdmin := false
	for _, role := range roles {
		if strings.ToUpper(strings.TrimSpace(role)) == SuperAdminRole {
			isSuperAdmin = true
			break
		}
	}

	var rpcResp *pb.GetUserPlatformsResp

	if isSuperAdmin {
		// 超级管理员：返回所有启用的平台
		l.Infof("检测到超级管理员角色，返回所有启用的平台: userId=%d", userId)
		allPlatformsResp, err := l.svcCtx.PortalRpc.PlatformGetAllEnabled(l.ctx, &pb.GetAllEnabledPlatformsReq{})
		if err != nil {
			l.Errorf("获取所有启用平台失败: userId=%d, error=%v", userId, err)
			return nil, err
		}
		// 转换为 GetUserPlatformsResp 格式
		rpcResp = &pb.GetUserPlatformsResp{
			Data: allPlatformsResp.Data,
		}
	} else {
		// 普通用户：查询用户平台表
		rpcResp, err = l.svcCtx.PortalRpc.UserPlatformGet(l.ctx, &pb.GetUserPlatformsReq{})
		if err != nil {
			l.Errorf("获取用户平台列表失败: userId=%d, error=%v", userId, err)
			return nil, err
		}
	}

	// 转换为 API 响应格式
	var platforms []types.SysPlatform
	for _, platform := range rpcResp.Data {
		platforms = append(platforms, types.SysPlatform{
			Id:           platform.Id,
			PlatformCode: platform.PlatformCode,
			PlatformName: platform.PlatformName,
			PlatformDesc: platform.PlatformDesc,
			PlatformIcon: platform.PlatformIcon,
			Sort:         platform.Sort,
			IsEnable:     platform.IsEnable,
			IsDefault:    platform.IsDefault,
			CreatedBy:    platform.CreateBy,
			UpdatedBy:    platform.UpdateBy,
			CreatedAt:    platform.CreateTime,
			UpdatedAt:    platform.UpdateTime,
		})
	}

	l.Infof("获取用户平台列表成功: userId=%d, isSuperAdmin=%v, platformCount=%d", userId, isSuperAdmin, len(platforms))
	return platforms, nil
}
