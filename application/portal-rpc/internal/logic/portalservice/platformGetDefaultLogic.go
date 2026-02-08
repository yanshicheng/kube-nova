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

type PlatformGetDefaultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformGetDefaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformGetDefaultLogic {
	return &PlatformGetDefaultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformGetDefault 获取默认平台
func (l *PlatformGetDefaultLogic) PlatformGetDefault(in *pb.GetDefaultSysPlatformReq) (*pb.GetDefaultSysPlatformResp, error) {
	// 查询默认平台
	platforms, err := l.svcCtx.SysPlatformModel.SearchNoPage(l.ctx, "sort", true, "`is_default` = ? AND `is_enable` = ?", 1, 1)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("获取默认平台失败：未找到默认平台")
			return nil, errorx.Msg("未找到默认平台")
		}
		l.Errorf("查询默认平台失败: %v", err)
		return nil, errorx.Msg("查询默认平台失败")
	}

	if len(platforms) == 0 {
		l.Errorf("获取默认平台失败：未找到默认平台")
		return nil, errorx.Msg("未找到默认平台")
	}

	// 取第一个默认平台（理论上只有一个）
	platform := platforms[0]

	// 转换为 protobuf 格式
	pbPlatform := &pb.SysPlatform{
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

	l.Infof("获取默认平台成功，平台ID: %d, 平台名称: %s", platform.Id, platform.PlatformName)
	return &pb.GetDefaultSysPlatformResp{
		Data: pbPlatform,
	}, nil
}
