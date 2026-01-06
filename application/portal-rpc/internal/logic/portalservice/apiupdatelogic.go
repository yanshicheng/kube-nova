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

// APIUpdate 更新 API 权限
func (l *APIUpdateLogic) APIUpdate(in *pb.UpdateSysAPIReq) (*pb.UpdateSysAPIResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("API ID 不能为空")
		return nil, errorx.Msg("API ID 不能为空")
	}
	if in.Name == "" {
		l.Error("API 名称不能为空")
		return nil, errorx.Msg("API 名称不能为空")
	}

	// 验证 API 是否存在
	existApi, err := l.svcCtx.SysApi.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("API 不存在，API ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("API 不存在")
	}

	// 如果修改了路径或方法，检查是否与其他 API 重复
	if in.Path != existApi.Path || in.Method != existApi.Method {
		duplicateApi, _ := l.svcCtx.SysApi.FindOneByPathMethod(l.ctx, in.Path, in.Method)
		if duplicateApi != nil && duplicateApi.Id != in.Id {
			l.Errorf("API 路径和方法组合已存在，路径: %s, 方法: %s", in.Path, in.Method)
			return nil, errorx.Msg("该 API 路径和方法组合已存在")
		}
	}

	// 记录是否需要重载策略
	needReload := existApi.Path != in.Path ||
		existApi.Method != in.Method ||
		existApi.IsPermission != in.IsPermission

	// 更新 API 信息
	existApi.ParentId = in.ParentId
	existApi.Name = in.Name
	existApi.Path = in.Path
	existApi.Method = in.Method
	existApi.IsPermission = in.IsPermission
	existApi.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysApi.Update(l.ctx, existApi)
	if err != nil {
		l.Errorf("更新 API 失败，API ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新 API 失败")
	}

	// 如果 API 的关键属性发生变化，需要重新加载权限策略
	if needReload && l.svcCtx.AuthzManager != nil {
		// 检查该 API 是否已被角色关联
		roleApis, err := l.svcCtx.SysRoleApi.SearchNoPage(l.ctx, "", true, "api_id = ?", in.Id)
		if err == nil && len(roleApis) > 0 {
			// API 已被角色关联，需要重载策略
			if err := l.svcCtx.AuthzManager.ReloadPolicy(l.ctx); err != nil {
				l.Errorf("重新加载权限策略失败: %v", err)
			} else {
				l.Infof("API 更新后权限策略重新加载成功，关联角色数量: %d", len(roleApis))
			}
		}
	}

	l.Infof("API 更新成功，API ID: %d, 名称: %s", in.Id, in.Name)
	return &pb.UpdateSysAPIResp{}, nil
}
