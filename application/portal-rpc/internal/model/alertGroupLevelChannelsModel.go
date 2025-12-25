package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertGroupLevelChannelsModel = (*customAlertGroupLevelChannelsModel)(nil)

type (
	// AlertGroupLevelChannelsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertGroupLevelChannelsModel.
	AlertGroupLevelChannelsModel interface {
		alertGroupLevelChannelsModel
		// 新增：删除指定group_id的所有缓存
		DeleteCacheByGroupId(ctx context.Context, groupId uint64) error
	}

	customAlertGroupLevelChannelsModel struct {
		*defaultAlertGroupLevelChannelsModel
	}
)

// NewAlertGroupLevelChannelsModel returns a model for the database table.
func NewAlertGroupLevelChannelsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertGroupLevelChannelsModel {
	return &customAlertGroupLevelChannelsModel{
		defaultAlertGroupLevelChannelsModel: newAlertGroupLevelChannelsModel(conn, c, opts...),
	}
}

// DeleteCacheByGroupId 删除指定group_id相关的所有缓存
func (m *customAlertGroupLevelChannelsModel) DeleteCacheByGroupId(ctx context.Context, groupId uint64) error {
	// 先查询所有该group_id的记录（在删除前查询）
	channels, err := m.SearchNoPage(ctx, "", true, "`group_id` = ?", groupId)
	if err != nil && err != ErrNotFound {
		return err
	}

	if len(channels) == 0 {
		return nil
	}

	// 收集所有缓存键并删除
	var cacheKeys []string
	for _, channel := range channels {
		cacheKeys = append(cacheKeys,
			fmt.Sprintf("%s%v", cacheIkubeopsAlertGroupLevelChannelsIdPrefix, channel.Id),
			fmt.Sprintf("%s%v:%v", cacheIkubeopsAlertGroupLevelChannelsGroupIdSeverityPrefix, channel.GroupId, channel.Severity),
		)
	}

	if len(cacheKeys) > 0 {
		return m.DelCache(cacheKeys...)
	}

	return nil
}
