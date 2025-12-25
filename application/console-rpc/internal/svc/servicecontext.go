package svc

import (
	"log"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	repository2 "github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/cluster"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                      config.Config
	Cache                       *redis.Redis
	HarborManager               *cluster.HarborManager
	ContainerRegistryModel      repository.ContainerRegistryModel
	RepositoryClusterModel      repository2.RegistryClusterModel
	RegistryProjectBindingModel repository2.RegistryProjectBindingModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn := sqlx.NewMysql(c.Mysql.DataSource)
	rawDB, err := sqlConn.RawDB()
	if err != nil {
		log.Fatal(err) // 处理错误
	}
	// 配置连接池参数
	rawDB.SetMaxOpenConns(c.Mysql.MaxOpenConns)       // 最大打开连接数
	rawDB.SetMaxIdleConns(c.Mysql.MaxIdleConns)       // 最大空闲连接数
	rawDB.SetConnMaxLifetime(c.Mysql.ConnMaxLifetime) // 连接的最大生命周期

	return &ServiceContext{
		Config: c,
		Cache:  redis.MustNewRedis(c.Cache),
		// BookModel: models.NewBooksModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
		HarborManager:               cluster.NewHarborManager(repository.NewContainerRegistryModel(sqlConn, c.DBCache)),
		ContainerRegistryModel:      repository.NewContainerRegistryModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
		RepositoryClusterModel:      repository2.NewRegistryClusterModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
		RegistryProjectBindingModel: repository2.NewRegistryProjectBindingModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
	}
}
