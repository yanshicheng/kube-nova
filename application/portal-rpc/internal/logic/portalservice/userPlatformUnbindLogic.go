package portalservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserPlatformUnbindLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserPlatformUnbindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPlatformUnbindLogic {
	return &UserPlatformUnbindLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserPlatformUnbind 解绑用户平台
func (l *UserPlatformUnbindLogic) UserPlatformUnbind(in *pb.UnbindUserPlatformReq) (*pb.UnbindUserPlatformResp, error) {
	// 参数验证
	if in.UserId <= 0 {
		l.Errorf("解绑用户平台失败：用户ID无效")
		return nil, errorx.Msg("用户ID无效")
	}
	if len(in.PlatformIds) == 0 {
		l.Errorf("解绑用户平台失败：平台ID列表不能为空")
		return nil, errorx.Msg("平台ID列表不能为空")
	}

	// 获取操作人信息
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	// 解绑指定的平台
	for _, platformId := range in.PlatformIds {
		// 查询用户平台绑定关系
		userPlatform, err := l.svcCtx.SysUserPlatformModel.FindOneByUserIdPlatformId(l.ctx, in.UserId, platformId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Infof("用户平台绑定不存在，跳过: userId=%d, platformId=%d", in.UserId, platformId)
				continue
			}
			l.Errorf("查询用户平台绑定失败: userId=%d, platformId=%d, error=%v", in.UserId, platformId, err)
			return nil, errorx.Msg("查询用户平台绑定失败")
		}

		// 如果已经删除，跳过
		if userPlatform.IsDeleted == 1 {
			l.Infof("用户平台绑定已删除，跳过: userId=%d, platformId=%d", in.UserId, platformId)
			continue
		}

		// 软删除
		userPlatform.IsDeleted = 1
		userPlatform.UpdateTime = time.Now()
		userPlatform.UpdateBy = sql.NullString{String: username, Valid: true}

		if err := l.svcCtx.SysUserPlatformModel.Update(l.ctx, userPlatform); err != nil {
			l.Errorf("解绑用户平台失败: userId=%d, platformId=%d, error=%v", in.UserId, platformId, err)
			return nil, errorx.Msg("解绑用户平台失败")
		}

		l.Infof("解绑用户平台成功: userId=%d, platformId=%d, operator=%s", in.UserId, platformId, username)
	}

	return &pb.UnbindUserPlatformResp{}, nil
}
