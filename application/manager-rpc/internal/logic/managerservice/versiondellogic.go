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

type VersionDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVersionDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionDelLogic {
	return &VersionDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VersionDel 删除版本（软删除）
func (l *VersionDelLogic) VersionDel(in *pb.DelOnecProjectVersionReq) (*pb.DelOnecProjectVersionResp, error) {
	// 参数校验
	if in.Id == 0 {
		l.Errorf("参数校验失败: id 不能为空")
		return nil, errorx.Msg("版本ID不能为空")
	}

	// 查询版本是否存在
	existVersion, err := l.svcCtx.OnecProjectVersion.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("版本不存在: id=%d", in.Id)
			return nil, errorx.Msg("版本不存在")
		}
		l.Errorf("查询版本失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询版本失败")
	}
	// TODO k8s 中删除对应的资源。
	// 执行软删除
	err = l.svcCtx.OnecProjectVersion.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除版本失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("删除版本失败")
	}

	l.Infof("版本删除成功: id=%d, applicationId=%d, version=%s",
		in.Id, existVersion.ApplicationId, existVersion.Version)

	return &pb.DelOnecProjectVersionResp{}, nil
}
