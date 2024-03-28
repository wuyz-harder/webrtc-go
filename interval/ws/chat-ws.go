package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"webrtc-go/interval/ws/message"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// 消息类型
type RTCMes struct {
	MesType string `json:"type"`
	Data    string `json:"data"`
	From    string `json:"from"`
	To      string `json:"to"`
}

// Node 当前用户节点 userId和Node的映射关系
type RtcNode struct {
	RtcClientID string `json:"rtcClientID"`
	// 这个是保留该node的wsid
	WsClientInfo message.WsClient `json:"wsClientInfo"`
	//这个是维护链接
	Conn *websocket.Conn `json:"-"`
	// 这个是消息队列
	DataQueue chan interface{} `json:"-"`
	// 群组的消息分发
}

var (
	HEART_BEAT = "HEART_BEAT"
	RTC_SDP    = "RTC_SDP"
	SEND_MSG   = "SEND_MSG"
	GET_MSG    = "GET_MSG"
)

// 映射关系表
var rtcClientMap map[string]*RtcNode = make(map[string]*RtcNode, 0)

type RtcSdpData struct {
	Sdp  string `json:"sdp"`
	Type string `json:"type"`
}

var upGrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 根据判断token的方法来鉴权,如果没token就返回false
		return true
	},
}

var rwLocker sync.RWMutex

func RtcChat(ctx *gin.Context) {
	// 升级协议以后原来的请求头会消失，所以要在query里用来获取
	// 在响应头上添加Sec-Websocket-Protocol,
	upGrader.Subprotocols = []string{ctx.GetHeader("Sec-Websocket-Protocol")}
	//升级get请求为webSocket协议
	fmt.Println(ctx.GetHeader("Sec-Websocket-Protocol"))
	conn, err := upGrader.Upgrade(ctx.Writer, ctx.Request, nil)
	userNumber := ctx.Query("userNumber")
	fmt.Println(userNumber)
	if err != nil {
		fmt.Println(err)
		return
	}
	// 绑定到当前节点
	rwLocker.Lock()
	_, exists := rtcClientMap[userNumber]

	// 判断是不是首次登录，不是首次的话之前应该是有相应的记录的
	if exists {
		// 更新的东西  1、状态  2、双方的连接
		rtcClientMap[userNumber].WsClientInfo.Online = true
		rtcClientMap[userNumber].Conn = conn
		rtcClientMap[userNumber].DataQueue = make(chan interface{}, 50)
	} else {
		var info message.WsClient
		info.Online = true
		node := &RtcNode{
			RtcClientID:  userNumber,
			WsClientInfo: info,
			Conn:         conn,
			DataQueue:    make(chan interface{}, 50),
		}
		// 映射关系的绑定
		rtcClientMap[userNumber] = node
	}
	// 广播更新用户列表
	// broadcast()
	rwLocker.Unlock()
	sendMsg(userNumber, map[string]interface{}{
		"msg":        "init success",
		"userNumber": userNumber,
		"type":       "LOGIN_SUCCESS",
		"users":      rtcClientMap,
	})
	// 发送数据给客户端
	go sendProc(rtcClientMap[userNumber])
	// 接收消息
	go listenFromClient(rtcClientMap[userNumber])
}

/*
*

	监听客户端的socket是否有发送消息过来，并处理发送的消息

*
*/
func listenFromClient(node *RtcNode) {
	for {
		_, mes, err := node.Conn.ReadMessage()

		if err != nil {
			node.Conn.WriteJSON(map[string]interface{}{"type": "error", "msg": err})
			return
		}
		var resMes RTCMes
		json.Unmarshal(mes, &resMes)

		switch resMes.MesType {
		// 心跳包处理
		case HEART_BEAT:
			// 给本人发送pong消息
			node.DataQueue <- (map[string]interface{}{"type": HEART_BEAT, "msg": "pong"})
			// sdp信息交换，由于webrtc
		case RTC_SDP:
			// 给远端发送消息
			rtcClientMap[resMes.To].DataQueue <- (map[string]interface{}{
				"from": resMes.From,
				"to":   resMes.To,
				"type": RTC_SDP,
				"data": resMes.Data,
			})
		case message.Offer:
			fmt.Println("offer来了！！！")
			rtcClientMap[resMes.To].DataQueue <- (map[string]interface{}{
				"from": resMes.From,
				"type": message.Offer,
				"to":   resMes.To,
				"data": resMes.Data,
			})

		case message.Answer:
			fmt.Println("Answer")
			fmt.Println(resMes)
			rtcClientMap[resMes.To].DataQueue <- (map[string]interface{}{
				"from": resMes.From,
				"to":   resMes.To,
				"type": message.Answer,
				"data": resMes.Data,
			})
			// 发送消息处理
		case message.HangUp:
			fmt.Println("HangUP")
			rtcClientMap[resMes.To].DataQueue <- (map[string]interface{}{
				"to":   resMes.To,
				"type": message.HangUp,
			})
			// 发送消息处理
		case message.Candidate:
			fmt.Println("Candidate")
			fmt.Println(resMes)
			rtcClientMap[resMes.To].DataQueue <- (map[string]interface{}{
				"from": resMes.From,
				"to":   resMes.To,
				"type": message.Candidate,
				"data": resMes.Data,
			})

		case SEND_MSG:
			// 发送消息给某个用户,判断该用户是否还在
			tmpNode, Nerr := rtcClientMap[resMes.To]
			if err != nil {
				// 给发送者回一条消息
				node.DataQueue <- map[string]interface{}{
					"type":    GET_MSG,
					"to":      resMes.To,
					"message": "发送失败",
					"from":    0,
					"msg":     "数据保存失败",
				}
			}
			// 如果用户已经
			if !Nerr {
				// 给发送者回一条消息
				node.DataQueue <- map[string]interface{}{
					"type": GET_MSG,
					"to":   resMes.To,
					"from": 0,
					"msg":  "该用户已下线",
				}
			} else {
				node.DataQueue <- map[string]interface{}{
					"type":    GET_MSG,
					"to":      resMes.To,
					"from":    resMes.From,
					"message": "",
					"msg":     "success",
				}
				tmpNode.DataQueue <- map[string]interface{}{
					"type":    GET_MSG,
					"to":      resMes.To,
					"from":    resMes.From,
					"message": "",
					"msg":     resMes.Data,
				}
			}
		case message.TextMessage:
			tmpNode, Nerr := rtcClientMap[resMes.To]
			if !Nerr {
				// 给发送者回一条消息
				node.DataQueue <- map[string]interface{}{
					"type":    GET_MSG,
					"to":      resMes.To,
					"message": "发送失败",
					"from":    0,
					"msg":     "数据保存失败",
				}
			} else {
				node.DataQueue <- map[string]interface{}{
					"type": message.TextMessage,
					"to":   resMes.To,
					"from": resMes.From,
					"data": resMes.Data,
				}
				tmpNode.DataQueue <- map[string]interface{}{
					"type": message.TextMessage,
					"to":   resMes.To,
					"from": resMes.From,
					"data": resMes.Data,
				}

			}

		}
	}
}

// 将数据推送到管道中
func sendMsg(userNumber string, message interface{}) {
	rwLocker.RLock()
	node, isOk := rtcClientMap[userNumber]
	fmt.Println(node)
	rwLocker.RUnlock()
	if isOk {
		node.DataQueue <- message
	}
}

// 给该用户的数据都都会发到所属管道里，管道中获取数据发送该用户
// 心跳保活机制，如果是时钟先到就结束了该函数
func sendProc(node *RtcNode) {
	timer := time.NewTicker(5 * time.Second) // 5s后触发
	// 加锁保护连接关闭操作
	rwLocker.Lock()
	conn := node.Conn
	rwLocker.Unlock()
	// 无限循环
EXIT:
	for {
		// 看看是时钟先到还是心跳先到
		select {
		case data := <-node.DataQueue:
			err := node.Conn.WriteJSON(data)
			if err != nil {
				fmt.Println(err)
			} else {
				timer.Stop()
				timer.Reset(5 * time.Second)
			}

		// 判断有没有超时保活
		case <-timer.C:
			timer.Stop()
			// 超时了就把这个链接关闭，然后置为下线,
			// node.Conn == conn这个是为了防止用户刷新了页面,导致conn已经被更换
			rwLocker.Lock()
			if node.Conn == conn {
				fmt.Printf("%s已关闭", node.RtcClientID)
				conn.Close()
				rtcClientMap[node.RtcClientID].WsClientInfo.Online = true
				broadcast()
			}
			rwLocker.Unlock()
			// 退出这个循环，因为用户都断线了
			break EXIT
		}
	}

}

// 广播更新列表
func broadcast() {
	for _, v := range rtcClientMap {
		// 在线的才发送
		if v.WsClientInfo.Online {
			v.DataQueue <- map[string]interface{}{
				"type":  "UPDATE_USER_LIST",
				"users": rtcClientMap,
			}
		}

	}
}
