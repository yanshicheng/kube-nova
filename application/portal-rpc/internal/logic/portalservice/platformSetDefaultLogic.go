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

type PlatformSetDefaultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformSetDefaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformSetDefaultLogic {
	return &PlatformSetDefaultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformSetDefault 设置为默认平台
func (l *PlatformSetDefaultLogic) PlatformSetDefault(in *pb.SetDefaultSysPlatformReq) (*pb.SetDefaultSysPlatformResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("设置默认平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("设置默认平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 检查平台是否已是默认平台
	if platform.IsDefault == 1 {
		l.Infof("平台已是默认平台，无需重复操作, id: %d", in.Id)
		return &pb.SetDefaultSysPlatformResp{}, nil
	}

	// 先取消其他平台的默认状态
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

	// 设置当前平台为默认平台
	platform.IsDefault = 1
	platform.UpdateTime = time.Now()
	if in.UpdateBy != "" {
		platform.UpdateBy = sql.NullString{String: in.UpdateBy, Valid: true}
	}

	// 更新数据库
	if err := l.svcCtx.SysPlatformModel.Update(l.ctx, platform); err != nil {
		l.Errorf("设置默认平台失败: %v", err)
		return nil, errorx.Msg("设置默认平台失败")
	}

	l.Infof("设置默认平台成功，平台ID: %d", in.Id)
	return &pb.SetDefaultSysPlatformResp{}, nil
}
