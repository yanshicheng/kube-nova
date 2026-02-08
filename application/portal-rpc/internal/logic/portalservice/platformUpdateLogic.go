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

type PlatformUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformUpdateLogic {
	return &PlatformUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformUpdate 更新平台
func (l *PlatformUpdateLogic) PlatformUpdate(in *pb.UpdateSysPlatformReq) (*pb.UpdateSysPlatformResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("更新平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("更新平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 如果修改了平台编码，检查新编码是否已存在
	if in.PlatformCode != "" && in.PlatformCode != platform.PlatformCode {
		existPlatform, err := l.svcCtx.SysPlatformModel.FindOneByPlatformCode(l.ctx, in.PlatformCode)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询平台编码失败: %v", err)
			return nil, errorx.Msg("查询平台编码失败")
		}
		if existPlatform != nil {
			l.Errorf("更新平台失败：平台编码已存在, code: %s", in.PlatformCode)
			return nil, errorx.Msg("平台编码已存在")
		}
		platform.PlatformCode = in.PlatformCode
	}

	// 如果设置为默认平台，需要先取消其他平台的默认状态
	if in.IsDefault == 1 && platform.IsDefault != 1 {
		// 查询当前默认平台
		defaultPlatforms, err := l.svcCtx.SysPlatformModel.SearchNoPage(l.ctx, "", true, "`is_default` = ? AND `id` != ?", 1, in.Id)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询默认平台失败: %v", err)
			return nil, errorx.Msg("查询默认平台失败")
		}

		// 取消其他平台的默认状态
		for _, p := range defaultPlatforms {
			p.IsDefault = 0
			p.UpdateTime = time.Now()
			if in.UpdateBy != "" {
				p.UpdateBy = sql.NullString{String: in.UpdateBy, Valid: true}
			}
			if err := l.svcCtx.SysPlatformModel.Update(l.ctx, p); err != nil {
				l.Errorf("取消默认平台失败: %v", err)
				return nil, errorx.Msg("取消默认平台失败")
			}
		}
	}

	// 更新字段
	if in.PlatformName != "" {
		platform.PlatformName = in.PlatformName
	}
	if in.PlatformDesc != "" {
		platform.PlatformDesc = in.PlatformDesc
	}
	if in.PlatformIcon != "" {
		platform.PlatformIcon = in.PlatformIcon
	}
	if in.Sort >= 0 {
		platform.Sort = in.Sort
	}
	platform.IsEnable = in.IsEnable
	platform.IsDefault = in.IsDefault

	platform.UpdateTime = time.Now()
	if in.UpdateBy != "" {
		platform.UpdateBy = sql.NullString{String: in.UpdateBy, Valid: true}
	}

	// 更新数据库
	if err := l.svcCtx.SysPlatformModel.Update(l.ctx, platform); err != nil {
		l.Errorf("更新平台失败: %v", err)
		return nil, errorx.Msg("更新平台失败")
	}

	l.Infof("更新平台成功，平台ID: %d", in.Id)
	return &pb.UpdateSysPlatformResp{}, nil
}
