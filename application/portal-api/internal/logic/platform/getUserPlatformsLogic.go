// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

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

func (l *GetUserPlatformsLogic) GetUserPlatforms(req *types.GetUserPlatformsRequest) (resp []types.SysPlatform, err error) {
	// 获取上下文中的用户ID
	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		l.Errorf("获取用户平台列表失败：用户ID无效")
		return nil, fmt.Errorf("用户ID无效")
	}

	rpcResp, err := l.svcCtx.PortalRpc.UserPlatformGet(l.ctx, &pb.GetUserPlatformsReq{
		ProjectId: req.ProjectId,
	})
	if err != nil {
		l.Errorf("获取用户平台列表失败: userId=%d, projectId=%d, error=%v", userId, req.ProjectId, err)
		return nil, err
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

	l.Infof("获取用户平台列表成功: userId=%d, projectId=%d, platformCount=%d", userId, req.ProjectId, len(platforms))
	return platforms, nil
}
