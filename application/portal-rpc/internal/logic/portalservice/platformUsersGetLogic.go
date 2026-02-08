package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlatformUsersGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformUsersGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformUsersGetLogic {
	return &PlatformUsersGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformUsersGet 获取平台下的用户列表
func (l *PlatformUsersGetLogic) PlatformUsersGet(in *pb.GetPlatformUsersReq) (*pb.GetPlatformUsersResp, error) {
	// 参数验证
	if in.PlatformId <= 0 {
		l.Errorf("获取平台用户列表失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 设置默认分页参数
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	// 验证平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.PlatformId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("获取平台用户列表失败：平台不存在, platformId=%d", in.PlatformId)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: platformId=%d, error=%v", in.PlatformId, err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 查询平台下的用户绑定关系（分页）
	userPlatforms, total, err := l.svcCtx.SysUserPlatformModel.Search(l.ctx, "id", false, page, pageSize, "`platform_id` = ?", in.PlatformId)
	if err != nil {
		l.Errorf("查询平台用户绑定失败: platformId=%d, error=%v", in.PlatformId, err)
		return nil, errorx.Msg("查询平台用户绑定失败")
	}

	// 如果没有用户，返回空列表
	if len(userPlatforms) == 0 {
		l.Infof("平台下没有用户: platformId=%d, platformName=%s", in.PlatformId, platform.PlatformName)
		return &pb.GetPlatformUsersResp{
			UserIds: []uint64{},
			Total:   0,
		}, nil
	}

	// 提取用户ID列表
	var userIds []uint64
	for _, up := range userPlatforms {
		userIds = append(userIds, up.UserId)
	}

	l.Infof("获取平台用户列表成功: platformId=%d, platformName=%s, userCount=%d, total=%d", in.PlatformId, platform.PlatformName, len(userIds), total)
	return &pb.GetPlatformUsersResp{
		UserIds: userIds,
		Total:   total,
	}, nil
}
