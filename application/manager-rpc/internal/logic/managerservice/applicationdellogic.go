package managerservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplicationDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationDelLogic {
	return &ApplicationDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApplicationDel 删除应用（软删除，级联删除版本）
func (l *ApplicationDelLogic) ApplicationDel(in *pb.DelOnecProjectApplicationReq) (*pb.DelOnecProjectApplicationResp, error) {
	// ========== 1. 参数校验 ==========
	if in.Id == 0 {
		l.Errorf("参数校验失败: id 不能为空")
		return nil, errorx.Msg("应用ID不能为空")
	}

	// ========== 2. 查询应用是否存在 ==========
	existApp, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用不存在: id=%d", in.Id)
			return nil, errorx.Msg("应用不存在")
		}
		l.Errorf("查询应用失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询应用失败")
	}

	l.Infof("开始删除应用: id=%d, nameEn=%s, nameCn=%s", in.Id, existApp.NameEn, existApp.NameCn)

	// ========== 3. 查询应用下的所有版本 ==========
	queryStr := "application_id = ?"
	versions, err := l.svcCtx.OnecProjectVersion.SearchNoPage(
		l.ctx,
		"id",     // orderStr: 按 id 排序
		false,    // isAsc: 降序
		queryStr, // queryStr: 查询条件
		in.Id,    // args: 应用ID
	)

	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询应用版本失败: %v, applicationId=%d", err, in.Id)
		return nil, errorx.Msg("查询应用版本失败")
	}

	versionCount := len(versions)
	if versionCount > 0 {
		l.Infof("应用下有 %d 个版本，开始无法删除应用，请先删除版本", versionCount)
		return nil, errorx.Msg("应用下有版本，请先删除版本")
	}
	err = l.svcCtx.OnecProjectApplication.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除应用失败: %v, id=%d", err, in.Id)
		return nil, fmt.Errorf("删除应用失败: %w", err)
	}

	l.Infof("应用删除成功: id=%d", in.Id)
	return &pb.DelOnecProjectApplicationResp{}, nil
}
