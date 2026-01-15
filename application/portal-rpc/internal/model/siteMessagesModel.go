package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SiteMessagesModel = (*customSiteMessagesModel)(nil)

type (
	// SiteMessagesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSiteMessagesModel.
	SiteMessagesModel interface {
		siteMessagesModel
		CountByUserId(ctx context.Context, userId uint64) (total, read, unread int64, err error)
	}

	customSiteMessagesModel struct {
		*defaultSiteMessagesModel
	}
)

func NewSiteMessagesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SiteMessagesModel {
	return &customSiteMessagesModel{
		defaultSiteMessagesModel: newSiteMessagesModel(conn, c, opts...),
	}
}

// CountByUserId 使用单条 SQL 获取用户消息的总数、已读数、未读数
func (m *customSiteMessagesModel) CountByUserId(ctx context.Context, userId uint64) (total, read, unread int64, err error) {
	var result struct {
		Total  int64 `db:"total"`
		Read   int64 `db:"read_count"`
		Unread int64 `db:"unread_count"`
	}

	query := `SELECT 
		COUNT(*) as total,
		COALESCE(SUM(CASE WHEN is_read = 1 THEN 1 ELSE 0 END), 0) as read_count,
		COALESCE(SUM(CASE WHEN is_read = 0 THEN 1 ELSE 0 END), 0) as unread_count
	FROM site_messages 
	WHERE user_id = ? AND is_deleted = 0`

	// 统计查询不需要走缓存，数据实时性要求高
	err = m.QueryRowNoCacheCtx(ctx, &result, query, userId)
	if err != nil {
		return 0, 0, 0, err
	}

	return result.Total, result.Read, result.Unread, nil
}
