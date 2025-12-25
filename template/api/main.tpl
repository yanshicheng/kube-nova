// Code scaffolded by goctl. Safe to edit.
// goctl {{.version}}

package main

import (
	"flag"
	"fmt"
   "github.com/zeromicro/go-zero/rest/httpx"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/handler/okx"
    middlewarex "github.com/yanshicheng/kube-nova/common/middleware"
	{{.importPackages}}
)

var configFile = flag.String("f", "etc/{{.serviceName}}.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c,conf.UseEnv())

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
