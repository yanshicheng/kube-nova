package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type VersionAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVersionAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionAddLogic {
	return &VersionAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VersionAdd 添加应用版本
func (l *VersionAddLogic) VersionAdd(in *pb.AddOnecProjectVersionReq) (*pb.AddOnecProjectVersionResp, error) {
	// 参数校验
	if in.ApplicationId == 0 {
		l.Errorf("参数校验失败: applicationId 不能为空")
		return nil, errorx.Msg("应用ID不能为空")
	}
	if in.Version == "" {
		l.Errorf("参数校验失败: version 不能为空")
		return nil, errorx.Msg("版本名称不能为空")
	}
	// 检查 version 是否合法。
	// TODO
	// 验证应用是否存在
	_, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, in.ApplicationId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用不存在: applicationId=%d", in.ApplicationId)
			return nil, errorx.Msg("应用不存在")
		}
		l.Errorf("查询应用失败: %v, applicationId=%d", err, in.ApplicationId)
		return nil, errorx.Msg("查询应用失败")
	}

	// 检查版本是否已存在
	existVersion, err := l.svcCtx.OnecProjectVersion.FindOneByApplicationIdVersion(
		l.ctx, in.ApplicationId, in.Version)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询版本失败: %v", err)
		return nil, errorx.Msg("查询版本失败")
	}
	if existVersion != nil {
		l.Errorf("版本已存在: applicationId=%d, version=%s", in.ApplicationId, in.Version)
		return nil, errorx.Msg("该应用下已存在相同版本")
	}

	// 将 map[string]string 转换为 JSON 字符串存储

	version := &model.OnecProjectVersion{
		ApplicationId: in.ApplicationId,
		Version:       in.Version,
		ResourceName:  in.ResourceName,
		CreatedBy:     in.CreatedBy,
		UpdatedBy:     in.UpdatedBy,
	}

	// 插入数据库
	result, err := l.svcCtx.OnecProjectVersion.Insert(l.ctx, version)
	if err != nil {
		l.Errorf("添加版本失败: %v, data: %+v", err, version)
		return nil, errorx.Msg("添加版本失败")
	}

	// 获取插入的ID
	id, err := result.LastInsertId()
	if err != nil {
		l.Errorf("获取版本ID失败: %v", err)
	} else {
		l.Infof("版本添加成功: id=%d, applicationId=%d, version=%s",
			id, in.ApplicationId, in.Version)
	}

	return &pb.AddOnecProjectVersionResp{
		Id: uint64(id),
	}, nil
}
