package svc

import (
    {{.imports}}
    "github.com/zeromicro/go-zero/core/stores/sqlx"
    "github.com/zeromicro/go-zero/core/stores/redis"
    "log"
)

type ServiceContext struct {
	Config config.Config
	Cache  *redis.Redis

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
		Config:c,
        Cache:redis.MustNewRedis(c.Cache),
        // BookModel: models.NewBooksModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
	}
}
