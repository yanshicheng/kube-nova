package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertGroupAppsModel = (*customAlertGroupAppsModel)(nil)

type (
	// AlertGroupAppsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertGroupAppsModel.
	AlertGroupAppsModel interface {
		alertGroupAppsModel
		// 新增：删除指定group_id的所有缓存
		DeleteCacheByGroupId(ctx context.Context, groupId uint64) error
	}

	customAlertGroupAppsModel struct {
		*defaultAlertGroupAppsModel
	}
)

// NewAlertGroupAppsModel returns a model for the database table.
func NewAlertGroupAppsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertGroupAppsModel {
	return &customAlertGroupAppsModel{
		defaultAlertGroupAppsModel: newAlertGroupAppsModel(conn, c, opts...),
	}
}

// DeleteCacheByGroupId 删除指定group_id相关的所有缓存
func (m *customAlertGroupAppsModel) DeleteCacheByGroupId(ctx context.Context, groupId uint64) error {
	// 先查询所有该group_id的记录（在删除前查询）
	apps, err := m.SearchNoPage(ctx, "", true, "`group_id` = ?", groupId)
	if err != nil && err != ErrNotFound {
		return err
	}

	if len(apps) == 0 {
		return nil
	}

	// 收集所有缓存键并删除
	var cacheKeys []string
	for _, app := range apps {
		cacheKeys = append(cacheKeys,
			fmt.Sprintf("%s%v", cacheIkubeopsAlertGroupAppsIdPrefix, app.Id),
			fmt.Sprintf("%s%v:%v:%v", cacheIkubeopsAlertGroupAppsGroupIdAppIdAppTypePrefix, app.GroupId, app.AppId, app.AppType),
		)
	}

	if len(cacheKeys) > 0 {
		return m.DelCache(cacheKeys...)
	}

	return nil
}
