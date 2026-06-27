package logservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fakeClusterModel struct {
	findOneByUUID func(ctx context.Context, uuid string) (*model.OnecCluster, error)
}

func (f *fakeClusterModel) Insert(ctx context.Context, data *model.OnecCluster) (sql.Result, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) FindOne(ctx context.Context, id uint64) (*model.OnecCluster, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) Search(ctx context.Context, orderStr string, isAsc bool, page, pageSize uint64, queryStr string, args ...any) ([]*model.OnecCluster, uint64, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) SearchNoPage(ctx context.Context, orderStr string, isAsc bool, queryStr string, args ...any) ([]*model.OnecCluster, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) FindOneByName(ctx context.Context, name string) (*model.OnecCluster, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) FindOneByUuid(ctx context.Context, uuid string) (*model.OnecCluster, error) {
	return f.findOneByUUID(ctx, uuid)
}

func (f *fakeClusterModel) Update(ctx context.Context, data *model.OnecCluster) error {
	panic("not implemented")
}

func (f *fakeClusterModel) Delete(ctx context.Context, id uint64) error {
	panic("not implemented")
}

func (f *fakeClusterModel) DeleteSoft(ctx context.Context, id uint64) error {
	panic("not implemented")
}

func (f *fakeClusterModel) TransCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	panic("not implemented")
}

func (f *fakeClusterModel) TransOnSql(ctx context.Context, session sqlx.Session, id uint64, sqlStr string, args ...any) (sql.Result, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) ExecSql(ctx context.Context, id uint64, sqlStr string, args ...any) (sql.Result, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) GetAllClusters(ctx context.Context) ([]*model.OnecCluster, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) GetProjectClusterStatsByClusterResource(ctx context.Context, clusterResourceId uint64) (*model.ProjectClusterStats, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) GetProjectClusterStatsByClusterUuid(ctx context.Context, clusterUuid string) (*model.ProjectClusterStats, error) {
	panic("not implemented")
}

func (f *fakeClusterModel) SyncClusterResourceByResourceId(ctx context.Context, clusterResourceId uint64) error {
	panic("not implemented")
}

func (f *fakeClusterModel) SyncClusterResourceByUuid(ctx context.Context, clusterUuid string) error {
	panic("not implemented")
}

func (f *fakeClusterModel) SyncClusterResourceByClusterId(ctx context.Context, clusterId uint64) error {
	panic("not implemented")
}

func (f *fakeClusterModel) SyncAllClusters(ctx context.Context) error {
	panic("not implemented")
}

type fakeClusterAppModel struct {
	searchNoPage func(ctx context.Context, orderStr string, isAsc bool, queryStr string, args ...any) ([]*model.OnecClusterApp, error)
}

func (f *fakeClusterAppModel) Insert(ctx context.Context, data *model.OnecClusterApp) (sql.Result, error) {
	panic("not implemented")
}

func (f *fakeClusterAppModel) FindOne(ctx context.Context, id uint64) (*model.OnecClusterApp, error) {
	panic("not implemented")
}

func (f *fakeClusterAppModel) Search(ctx context.Context, orderStr string, isAsc bool, page, pageSize uint64, queryStr string, args ...any) ([]*model.OnecClusterApp, uint64, error) {
	panic("not implemented")
}

func (f *fakeClusterAppModel) SearchNoPage(ctx context.Context, orderStr string, isAsc bool, queryStr string, args ...any) ([]*model.OnecClusterApp, error) {
	return f.searchNoPage(ctx, orderStr, isAsc, queryStr, args...)
}

func (f *fakeClusterAppModel) FindOneByClusterUuidAppCodeAppType(ctx context.Context, clusterUuid string, appCode string, appType int64) (*model.OnecClusterApp, error) {
	panic("not implemented")
}

func (f *fakeClusterAppModel) Update(ctx context.Context, data *model.OnecClusterApp) error {
	panic("not implemented")
}

func (f *fakeClusterAppModel) Delete(ctx context.Context, id uint64) error {
	panic("not implemented")
}

func (f *fakeClusterAppModel) DeleteSoft(ctx context.Context, id uint64) error {
	panic("not implemented")
}

func (f *fakeClusterAppModel) TransCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	panic("not implemented")
}

func (f *fakeClusterAppModel) TransOnSql(ctx context.Context, session sqlx.Session, id uint64, sqlStr string, args ...any) (sql.Result, error) {
	panic("not implemented")
}

func (f *fakeClusterAppModel) ExecSql(ctx context.Context, id uint64, sqlStr string, args ...any) (sql.Result, error) {
	panic("not implemented")
}

func TestGetLogQueryModeOptionsRecognizesBackends(t *testing.T) {
	logic := NewGetLogQueryModeOptionsLogic(context.Background(), &svc.ServiceContext{
		OnecClusterModel: &fakeClusterModel{findOneByUUID: func(ctx context.Context, uuid string) (*model.OnecCluster, error) {
			return &model.OnecCluster{Uuid: uuid}, nil
		}},
		OnecClusterAppModel: &fakeClusterAppModel{searchNoPage: func(ctx context.Context, orderStr string, isAsc bool, queryStr string, args ...any) ([]*model.OnecClusterApp, error) {
			return []*model.OnecClusterApp{{AppCode: "loki"}, {AppCode: "elasticsearch"}, {AppCode: "es"}}, nil
		}},
	})

	resp, err := logic.GetLogQueryModeOptions(&pb.LogQueryModeOptionsReq{ClusterUuid: "cluster-1"})
	if err != nil {
		t.Fatalf("GetLogQueryModeOptions() returned error: %v", err)
	}
	if !resp.Loki || !resp.Elasticsearch {
		t.Fatalf("expected both backends available, got %+v", resp)
	}
}

func TestGetLogQueryModeOptionsReturnsFalseWhenNoLoggingApp(t *testing.T) {
	logic := NewGetLogQueryModeOptionsLogic(context.Background(), &svc.ServiceContext{
		OnecClusterModel: &fakeClusterModel{findOneByUUID: func(ctx context.Context, uuid string) (*model.OnecCluster, error) {
			return &model.OnecCluster{Uuid: uuid}, nil
		}},
		OnecClusterAppModel: &fakeClusterAppModel{searchNoPage: func(ctx context.Context, orderStr string, isAsc bool, queryStr string, args ...any) ([]*model.OnecClusterApp, error) {
			return nil, model.ErrNotFound
		}},
	})

	resp, err := logic.GetLogQueryModeOptions(&pb.LogQueryModeOptionsReq{ClusterUuid: "cluster-1"})
	if err != nil {
		t.Fatalf("GetLogQueryModeOptions() returned error: %v", err)
	}
	if resp.Loki || resp.Elasticsearch {
		t.Fatalf("expected both backends unavailable, got %+v", resp)
	}
}

func TestGetLogQueryModeOptionsReturnsClusterNotFound(t *testing.T) {
	logic := NewGetLogQueryModeOptionsLogic(context.Background(), &svc.ServiceContext{
		OnecClusterModel: &fakeClusterModel{findOneByUUID: func(ctx context.Context, uuid string) (*model.OnecCluster, error) {
			return nil, model.ErrNotFound
		}},
	})

	_, err := logic.GetLogQueryModeOptions(&pb.LogQueryModeOptionsReq{ClusterUuid: "cluster-1"})
	if err == nil || err.Error() != "指定的集群不存在" {
		t.Fatalf("expected cluster not found error, got %v", err)
	}
}

func TestGetLogQueryModeOptionsReturnsLookupError(t *testing.T) {
	logic := NewGetLogQueryModeOptionsLogic(context.Background(), &svc.ServiceContext{
		OnecClusterModel: &fakeClusterModel{findOneByUUID: func(ctx context.Context, uuid string) (*model.OnecCluster, error) {
			return &model.OnecCluster{Uuid: uuid}, nil
		}},
		OnecClusterAppModel: &fakeClusterAppModel{searchNoPage: func(ctx context.Context, orderStr string, isAsc bool, queryStr string, args ...any) ([]*model.OnecClusterApp, error) {
			return nil, errors.New("db error")
		}},
	})

	_, err := logic.GetLogQueryModeOptions(&pb.LogQueryModeOptionsReq{ClusterUuid: "cluster-1"})
	if err == nil || err.Error() != "查询集群应用列表失败" {
		t.Fatalf("expected app lookup error, got %v", err)
	}
}
