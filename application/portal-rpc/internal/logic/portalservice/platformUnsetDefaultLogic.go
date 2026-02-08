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

type PlatformUnsetDefaultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformUnsetDefaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformUnsetDefaultLogic {
	return &PlatformUnsetDefaultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformUnsetDefault 取消默认平台
func (l *PlatformUnsetDefaultLogic) PlatformUnsetDefault(in *pb.UnsetDefaultSysPlatformReq) (*pb.UnsetDefaultSysPlatformResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("取消默认平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("取消默认平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 检查平台是否为默认平台
	if platform.IsDefault == 0 {
		l.Infof("平台不是默认平台，无需取消操作, id: %d", in.Id)
		return &pb.UnsetDefaultSysPlatformResp{}, nil
	}

	// 取消默认状态
	platform.IsDefault = 0
	platform.UpdateTime = time.Now()
	if in.UpdateBy != "" {
		platform.UpdateBy = sql.NullString{String: in.UpdateBy, Valid: true}
	}

	// 更新数据库
	if err := l.svcCtx.SysPlatformModel.Update(l.ctx, platform); err != nil {
		l.Errorf("取消默认平台失败: %v", err)
		return nil, errorx.Msg("取消默认平台失败")
	}

	l.Infof("取消默认平台成功，平台ID: %d", in.Id)
	return &pb.UnsetDefaultSysPlatformResp{}, nil
}
