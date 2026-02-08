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

type PlatformAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAddLogic {
	return &PlatformAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformAdd 添加平台
func (l *PlatformAddLogic) PlatformAdd(in *pb.AddSysPlatformReq) (*pb.AddSysPlatformResp, error) {
	// 参数验证
	if in.PlatformCode == "" {
		l.Errorf("添加平台失败：平台编码不能为空")
		return nil, errorx.Msg("平台编码不能为空")
	}
	if in.PlatformName == "" {
		l.Errorf("添加平台失败：平台名称不能为空")
		return nil, errorx.Msg("平台名称不能为空")
	}

	// 检查平台编码是否已存在
	existPlatform, err := l.svcCtx.SysPlatformModel.FindOneByPlatformCode(l.ctx, in.PlatformCode)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询平台编码失败: %v", err)
		return nil, errorx.Msg("查询平台编码失败")
	}
	if existPlatform != nil {
		l.Errorf("添加平台失败：平台编码已存在, code: %s", in.PlatformCode)
		return nil, errorx.Msg("平台编码已存在")
	}

	// 如果设置为默认平台，需要先取消其他平台的默认状态
	if in.IsDefault == 1 {
		// 查询当前默认平台
		defaultPlatforms, err := l.svcCtx.SysPlatformModel.SearchNoPage(l.ctx, "", true, "`is_default` = ?", 1)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询默认平台失败: %v", err)
			return nil, errorx.Msg("查询默认平台失败")
		}

		// 取消其他平台的默认状态
		for _, platform := range defaultPlatforms {
			platform.IsDefault = 0
			platform.UpdateTime = time.Now()
			if in.CreateBy != "" {
				platform.UpdateBy = sql.NullString{String: in.CreateBy, Valid: true}
			}
			if err := l.svcCtx.SysPlatformModel.Update(l.ctx, platform); err != nil {
				l.Errorf("取消默认平台失败: %v", err)
				return nil, errorx.Msg("取消默认平台失败")
			}
		}
	}

	// 构建平台数据
	platform := &model.SysPlatform{
		PlatformCode: in.PlatformCode,
		PlatformName: in.PlatformName,
		PlatformDesc: in.PlatformDesc,
		PlatformIcon: in.PlatformIcon,
		Sort:         in.Sort,
		IsEnable:     in.IsEnable,
		IsDefault:    0,
		IsDeleted:    0,
		CreateTime:   time.Now(),
		UpdateTime:   time.Now(),
	}

	if in.CreateBy != "" {
		platform.CreateBy = sql.NullString{String: in.CreateBy, Valid: true}
		platform.UpdateBy = sql.NullString{String: in.CreateBy, Valid: true}
	}

	// 插入数据库
	_, err = l.svcCtx.SysPlatformModel.Insert(l.ctx, platform)
	if err != nil {
		l.Errorf("添加平台失败: %v", err)
		return nil, errorx.Msg("添加平台失败")
	}

	l.Infof("添加平台成功，平台编码: %s, 平台名称: %s", in.PlatformCode, in.PlatformName)
	return &pb.AddSysPlatformResp{}, nil
}
