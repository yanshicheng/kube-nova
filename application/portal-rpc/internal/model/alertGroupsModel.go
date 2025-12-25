package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertGroupsModel = (*customAlertGroupsModel)(nil)

type (
	// AlertGroupsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertGroupsModel.
	AlertGroupsModel interface {
		alertGroupsModel
		DeleteCacheById(ctx context.Context, id uint64, uuid string) error
	}

	customAlertGroupsModel struct {
		*defaultAlertGroupsModel
	}
)

// NewAlertGroupsModel returns a model for the database table.
func NewAlertGroupsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertGroupsModel {
	return &customAlertGroupsModel{
		defaultAlertGroupsModel: newAlertGroupsModel(conn, c, opts...),
	}
}

// DeleteCacheById 删除指定id的告警组缓存
func (m *customAlertGroupsModel) DeleteCacheById(ctx context.Context, id uint64, uuid string) error {
	alertGroupIdKey := fmt.Sprintf("%s%v", cacheIkubeopsAlertGroupsIdPrefix, id)
	alertGroupUuidKey := fmt.Sprintf("%s%v", cacheIkubeopsAlertGroupsUuidPrefix, uuid)
	return m.DelCache(alertGroupIdKey, alertGroupUuidKey)
}
