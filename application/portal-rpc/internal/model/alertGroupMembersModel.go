package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertGroupMembersModel = (*customAlertGroupMembersModel)(nil)

type (
	// AlertGroupMembersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAlertGroupMembersModel.
	AlertGroupMembersModel interface {
		alertGroupMembersModel
		// 新增：删除指定group_id的所有缓存
		DeleteCacheByGroupId(ctx context.Context, groupId uint64) error
	}

	customAlertGroupMembersModel struct {
		*defaultAlertGroupMembersModel
	}
)

// NewAlertGroupMembersModel returns a model for the database table.
func NewAlertGroupMembersModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertGroupMembersModel {
	return &customAlertGroupMembersModel{
		defaultAlertGroupMembersModel: newAlertGroupMembersModel(conn, c, opts...),
	}
}

// DeleteCacheByGroupId 删除指定group_id相关的所有缓存
func (m *customAlertGroupMembersModel) DeleteCacheByGroupId(ctx context.Context, groupId uint64) error {
	// 先查询所有该group_id的记录（在删除前查询）
	members, err := m.SearchNoPage(ctx, "", true, "`group_id` = ?", groupId)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	if len(members) == 0 {
		return nil
	}

	// 收集所有缓存键并删除
	var cacheKeys []string
	for _, member := range members {
		cacheKeys = append(cacheKeys,
			fmt.Sprintf("%s%v", cacheIkubeopsAlertGroupMembersIdPrefix, member.Id),
			fmt.Sprintf("%s%v:%v:%v", cacheIkubeopsAlertGroupMembersGroupIdUserIdIsDeletedPrefix, member.GroupId, member.UserId, member.IsDeleted),
		)
	}

	if len(cacheKeys) > 0 {
		return m.DelCache(cacheKeys...)
	}

	return nil
}
