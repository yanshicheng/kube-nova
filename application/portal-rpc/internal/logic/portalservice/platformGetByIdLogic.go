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

type PlatformGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformGetByIdLogic {
	return &PlatformGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformGetById 根据ID获取平台
func (l *PlatformGetByIdLogic) PlatformGetById(in *pb.GetSysPlatformByIdReq) (*pb.GetSysPlatformByIdResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("获取平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("获取平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

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

	return &pb.GetSysPlatformByIdResp{
		Data: pbPlatform,
	}, nil
}
