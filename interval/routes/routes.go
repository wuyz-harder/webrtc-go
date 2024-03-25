package routes

import (
	"webrtc-go/interval/ws"

	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine) {

	v1 := r.Group("/v1/api")
	// 聊天
	v1.Handle("GET", "/chat", ws.RtcChat)

}
