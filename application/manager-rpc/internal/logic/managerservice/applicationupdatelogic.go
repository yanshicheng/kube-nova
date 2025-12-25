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

type ApplicationUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplicationUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationUpdateLogic {
	return &ApplicationUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApplicationUpdate 更新应用信息（只允许修改中文名称和描述）
func (l *ApplicationUpdateLogic) ApplicationUpdate(in *pb.UpdateOnecProjectApplicationReq) (*pb.UpdateOnecProjectApplicationResp, error) {
	// 参数校验
	if in.Id == 0 {
		l.Errorf("参数校验失败: id 不能为空")
		return nil, errorx.Msg("应用ID不能为空")
	}

	// 查询应用是否存在
	existApp, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用不存在: id=%d", in.Id)
			return nil, errorx.Msg("应用不存在")
		}
		l.Errorf("查询应用失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询应用失败")
	}

	// 构建更新数据（只更新允许的字段）
	existApp.NameCn = in.NameCn
	existApp.Description = in.Description
	existApp.UpdatedBy = in.UpdatedBy

	// 更新数据库
	err = l.svcCtx.OnecProjectApplication.Update(l.ctx, existApp)
	if err != nil {
		l.Errorf("更新应用失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("更新应用失败")
	}

	return &pb.UpdateOnecProjectApplicationResp{}, nil
}
