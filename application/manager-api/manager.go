package main

import (
	"flag"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/config"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/handler"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/handler/okx"
	middlewarex "github.com/yanshicheng/kube-nova/common/middleware"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/manager-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 自定义全局中间件
	server.Use(middlewarex.PanicRecoveryMiddleware)

	// 自定义错误
	httpx.SetErrorHandler(errorx.ErrHandler)
	httpx.SetOkHandler(okx.OkHandler)

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
