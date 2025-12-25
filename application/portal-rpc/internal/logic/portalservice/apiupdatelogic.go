package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type APIUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPIUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APIUpdateLogic {
	return &APIUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// APIUpdate 更新API权限
func (l *APIUpdateLogic) APIUpdate(in *pb.UpdateSysAPIReq) (*pb.UpdateSysAPIResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("API ID不能为空")
		return nil, errorx.Msg("API ID不能为空")
	}
	if in.Name == "" {
		l.Error("API名称不能为空")
		return nil, errorx.Msg("API名称不能为空")
	}

	// 验证API是否存在
	existApi, err := l.svcCtx.SysApi.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("API不存在，API ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("API不存在")
	}

	// 如果修改了路径或方法，检查是否重复
	if in.Path != existApi.Path || in.Method != existApi.Method {
		duplicateApi, _ := l.svcCtx.SysApi.FindOneByPathMethod(l.ctx, in.Path, in.Method)
		if duplicateApi != nil && duplicateApi.Id != in.Id {
			l.Errorf("API路径和方法组合已存在，路径: %s, 方法: %s", in.Path, in.Method)
			return nil, errorx.Msg("该API路径和方法组合已存在")
		}
	}

	// 更新API信息
	existApi.ParentId = in.ParentId
	existApi.Name = in.Name
	existApi.Path = in.Path
	existApi.Method = in.Method
	existApi.IsPermission = in.IsPermission
	existApi.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysApi.Update(l.ctx, existApi)
	if err != nil {
		l.Errorf("更新API失败，API ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新API失败")
	}

	l.Infof("API更新成功，API ID: %d, 名称: %s", in.Id, in.Name)
	return &pb.UpdateSysAPIResp{}, nil
}
