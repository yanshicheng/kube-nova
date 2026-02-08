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

type PlatformDisableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformDisableLogic {
	return &PlatformDisableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformDisable 禁用平台
func (l *PlatformDisableLogic) PlatformDisable(in *pb.DisableSysPlatformReq) (*pb.DisableSysPlatformResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("禁用平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("禁用平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 检查是否为默认平台，默认平台不允许禁用
	if platform.IsDefault == 1 {
		l.Errorf("禁用平台失败：默认平台不允许禁用, id: %d", in.Id)
		return nil, errorx.Msg("默认平台不允许禁用，请先设置其他平台为默认平台")
	}

	// 检查平台是否已禁用
	if platform.IsEnable == 0 {
		l.Infof("平台已处于禁用状态，无需重复操作, id: %d", in.Id)
		return &pb.DisableSysPlatformResp{}, nil
	}

	// 更新禁用状态
	platform.IsEnable = 0
	platform.UpdateTime = time.Now()
	if in.UpdateBy != "" {
		platform.UpdateBy = sql.NullString{String: in.UpdateBy, Valid: true}
	}

	// 更新数据库
	if err := l.svcCtx.SysPlatformModel.Update(l.ctx, platform); err != nil {
		l.Errorf("禁用平台失败: %v", err)
		return nil, errorx.Msg("禁用平台失败")
	}

	l.Infof("禁用平台成功，平台ID: %d", in.Id)
	return &pb.DisableSysPlatformResp{}, nil
}
