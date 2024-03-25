package message

type WsClient struct {
	Online bool `json:"online"`
	// 后面这个是设置自动生成时间，默认是当前值
}

// 定义消息类型枚举
const (
	Offer              = "offer"
	Answer             = "answer"
	Candidate          = "candidate"
	CreateRoom         = "create"
	JoinRoom           = "join"
	LeaveRoom          = "leave"
	OfferRenegotiation = "offerRenegotiation"
)

// 定义消息结构体
type SignalingMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
