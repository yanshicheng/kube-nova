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

type PlatformEnableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformEnableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformEnableLogic {
	return &PlatformEnableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformEnable 启用平台
func (l *PlatformEnableLogic) PlatformEnable(in *pb.EnableSysPlatformReq) (*pb.EnableSysPlatformResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("启用平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("启用平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 检查平台是否已启用
	if platform.IsEnable == 1 {
		l.Infof("平台已处于启用状态，无需重复操作, id: %d", in.Id)
		return &pb.EnableSysPlatformResp{}, nil
	}

	// 更新启用状态
	platform.IsEnable = 1
	platform.UpdateTime = time.Now()
	if in.UpdateBy != "" {
		platform.UpdateBy = sql.NullString{String: in.UpdateBy, Valid: true}
	}

	// 更新数据库
	if err := l.svcCtx.SysPlatformModel.Update(l.ctx, platform); err != nil {
		l.Errorf("启用平台失败: %v", err)
		return nil, errorx.Msg("启用平台失败")
	}

	l.Infof("启用平台成功，平台ID: %d", in.Id)
	return &pb.EnableSysPlatformResp{}, nil
}
