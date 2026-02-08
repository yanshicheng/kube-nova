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

type PlatformGetAllEnabledLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformGetAllEnabledLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformGetAllEnabledLogic {
	return &PlatformGetAllEnabledLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformGetAllEnabled 获取所有启用的平台
func (l *PlatformGetAllEnabledLogic) PlatformGetAllEnabled(in *pb.GetAllEnabledPlatformsReq) (*pb.GetAllEnabledPlatformsResp, error) {
	// 查询所有启用的平台，按排序字段升序排列
	platforms, err := l.svcCtx.SysPlatformModel.SearchNoPage(l.ctx, "sort", true, "`is_enable` = ?", 1)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未找到启用的平台")
			return &pb.GetAllEnabledPlatformsResp{
				Data: []*pb.SysPlatform{},
			}, nil
		}
		l.Errorf("查询启用的平台失败: %v", err)
		return nil, errorx.Msg("查询启用的平台失败")
	}

	// 转换为 protobuf 格式
	var pbPlatforms []*pb.SysPlatform
	for _, platform := range platforms {
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
		pbPlatforms = append(pbPlatforms, pbPlatform)
	}

	l.Infof("获取所有启用的平台成功，共 %d 个平台", len(pbPlatforms))
	return &pb.GetAllEnabledPlatformsResp{
		Data: pbPlatforms,
	}, nil
}
