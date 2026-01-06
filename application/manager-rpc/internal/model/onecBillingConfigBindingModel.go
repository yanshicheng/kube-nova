package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecBillingConfigBindingModel = (*customOnecBillingConfigBindingModel)(nil)

type (
	OnecBillingConfigBindingModel interface {
		onecBillingConfigBindingModel
		FindByPriceConfigId(ctx context.Context, priceConfigId uint64) ([]*OnecBillingConfigBinding, error)
		Search(ctx context.Context, bindingType, clusterUuid string, projectId uint64) ([]*OnecBillingConfigBinding, error)
		FindByClusterAndProject(ctx context.Context, clusterUuid string, projectId uint64) (*OnecBillingConfigBinding, error)
	}

	customOnecBillingConfigBindingModel struct {
		*defaultOnecBillingConfigBindingModel
	}
)

func NewOnecBillingConfigBindingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecBillingConfigBindingModel {
	return &customOnecBillingConfigBindingModel{
		defaultOnecBillingConfigBindingModel: newOnecBillingConfigBindingModel(conn, c, opts...),
	}
}

// FindByPriceConfigId 根据价格配置ID查找绑定关系
func (m *customOnecBillingConfigBindingModel) FindByPriceConfigId(ctx context.Context, priceConfigId uint64) ([]*OnecBillingConfigBinding, error) {
	var resp []*OnecBillingConfigBinding
	query := fmt.Sprintf("select %s from %s where `price_config_id` = ? and `is_deleted` = 0", onecBillingConfigBindingRows, m.table)
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &resp, query, priceConfigId)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Search 搜索绑定关系
func (m *customOnecBillingConfigBindingModel) Search(ctx context.Context, bindingType, clusterUuid string, projectId uint64) ([]*OnecBillingConfigBinding, error) {
	var resp []*OnecBillingConfigBinding
	query := fmt.Sprintf("select %s from %s where `is_deleted` = 0", onecBillingConfigBindingRows, m.table)
	args := make([]interface{}, 0)

	if bindingType != "" {
		query += " and `binding_type` = ?"
		args = append(args, bindingType)
	}
	if clusterUuid != "" {
		query += " and `binding_cluster_uuid` = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		query += " and `binding_project_id` = ?"
		args = append(args, projectId)
	}

	query += " order by `id` desc"

	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &resp, query, args...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// FindByClusterAndProject 根据集群和项目查找绑定
func (m *customOnecBillingConfigBindingModel) FindByClusterAndProject(ctx context.Context, clusterUuid string, projectId uint64) (*OnecBillingConfigBinding, error) {
	var resp OnecBillingConfigBinding
	query := fmt.Sprintf("select %s from %s where `binding_cluster_uuid` = ? and `binding_project_id` = ? and `is_deleted` = 0 limit 1", onecBillingConfigBindingRows, m.table)
	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &resp, query, clusterUuid, projectId)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
