// ============ 1. BindAlertGroupApps 接口实现 ============
package alertservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindAlertGroupAppsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindAlertGroupAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindAlertGroupAppsLogic {
	return &BindAlertGroupAppsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BindAlertGroupApps 告警组关联app
func (l *BindAlertGroupAppsLogic) BindAlertGroupApps(in *pb.BindAlertGroupAppsReq) (*pb.BindAlertGroupAppsResp, error) {

	//支持的 appType 列表
	apps := []string{"project"}

	// 检查 	in.AppType 是不是支持的
	// 写一个函数
	if !l.contains(apps, in.AppType) {
		l.Errorf("不支持的 appType: %s", in.AppType)
		return nil, errorx.Msg("不支持的 appType")
	}
	// 1. 检查告警组是否存在
	_, err := l.svcCtx.AlertGroupsModel.FindOne(l.ctx, in.GroupId)
	if err != nil {
		l.Errorf("告警组不存在: groupId=%d, err=%v", in.GroupId, err)
		return nil, errorx.Msg("告警组不存在")
	}

	// 2. 检查是否已存在关联
	existData, err := l.svcCtx.AlertGroupAppsModel.FindOneByGroupIdAppIdAppType(l.ctx, in.GroupId, in.AppId, in.AppType)
	if err == nil && existData != nil {
		l.Infof("告警组与应用的关联已存在: groupId=%d, appId=%d, appType=%s", in.GroupId, in.AppId, in.AppType)
		return nil, errorx.Msg("告警组与应用的关联已存在")
	}

	// 3. 创建关联
	data := &model.AlertGroupApps{
		GroupId:   in.GroupId,
		AppId:     in.AppId,
		AppType:   in.AppType,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDeleted: 0,
	}

	_, err = l.svcCtx.AlertGroupAppsModel.Insert(l.ctx, data)
	if err != nil {
		l.Errorf("创建告警组应用关联失败: %v", err)
		return nil, errorx.Msg("创建告警组应用关联失败")
	}

	l.Infof("成功创建告警组应用关联: groupId=%d, appId=%d, appType=%s", in.GroupId, in.AppId, in.AppType)
	return &pb.BindAlertGroupAppsResp{}, nil
}

// contains
func (l *BindAlertGroupAppsLogic) contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
