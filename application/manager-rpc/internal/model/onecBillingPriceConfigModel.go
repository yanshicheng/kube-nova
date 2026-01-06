package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecBillingPriceConfigModel = (*customOnecBillingPriceConfigModel)(nil)

type (
	OnecBillingPriceConfigModel interface {
		onecBillingPriceConfigModel
		SearchByName(ctx context.Context, configName string) ([]*OnecBillingPriceConfig, error)
		FindAll(ctx context.Context) ([]*OnecBillingPriceConfig, error)
	}

	customOnecBillingPriceConfigModel struct {
		*defaultOnecBillingPriceConfigModel
	}
)

func NewOnecBillingPriceConfigModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecBillingPriceConfigModel {
	return &customOnecBillingPriceConfigModel{
		defaultOnecBillingPriceConfigModel: newOnecBillingPriceConfigModel(conn, c, opts...),
	}
}

// SearchByName 根据配置名称模糊搜索
func (m *customOnecBillingPriceConfigModel) SearchByName(ctx context.Context, configName string) ([]*OnecBillingPriceConfig, error) {
	var resp []*OnecBillingPriceConfig
	query := fmt.Sprintf("select %s from %s where `is_deleted` = 0", onecBillingPriceConfigRows, m.table)
	args := make([]interface{}, 0)

	if configName != "" {
		query += " and `config_name` like ?"
		args = append(args, "%"+configName+"%")
	}

	query += " order by `id` desc"

	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &resp, query, args...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// FindAll 查询所有未删除的配置
func (m *customOnecBillingPriceConfigModel) FindAll(ctx context.Context) ([]*OnecBillingPriceConfig, error) {
	var resp []*OnecBillingPriceConfig
	query := fmt.Sprintf("select %s from %s where `is_deleted` = 0 order by `id` desc", onecBillingPriceConfigRows, m.table)
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &resp, query)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
