package main

import (
	"fmt"
	cors2 "webrtc-go/interval/cors"
	"webrtc-go/interval/routes"
	_ "webrtc-go/interval/ws/message"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-ini/ini"
)

func main() {

	cfg, err := ini.Load("../config/config.ini")
	if err != nil {
		fmt.Printf("Failed to read configuration file: %v", err)
		return
	}

	port := cfg.Section("server").Key("port").String()
	// cert := cfg.Section("server").Key("cert").String()
	// key := cfg.Section("server").Key("key").String()
	r := gin.New()
	//  中间件
	r.Use(cors.New(cors2.GetCors()))
	// 错误拦截器
	r.MaxMultipartMemory = 8 << 20
	//跨域设置
	routes.Routes(r)
	runErr := r.Run(port)
	// 下面是https
	// runErr := r.RunTLS(port, cert, key)
	if runErr != nil {
		fmt.Println("启动出错了")
		return
	}

}
