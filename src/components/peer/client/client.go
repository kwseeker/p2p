package client

import (
	"github.com/pion/webrtc/v4"
	"log"
	"sync"
)

type Option struct {
	SignalServerAddr string // 信令服务器地址
	ICEServerAddr    string // ICE服务器地址
	PeerType         string // Peer类型，可选值："offer"、"answer"
}

// Client Peer 节点在P2P连接建立前只会和信令服务器和ICE服务器进行通信
type Client struct {
	Option
	peerConn      *webrtc.PeerConnection
	candidatesMux sync.Mutex
}

func NewClient(option *Option) *Client {
	return &Client{
		Option: *option,
	}
}

// Run Peer 节点启动
func (c *Client) Run() {
	// 连接ICE服务器
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{c.ICEServerAddr},
			},
		},
	}
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Fatalln(err)
	}
	c.peerConn = peerConnection
	defer func() {
		if err := peerConnection.Close(); err != nil {
			log.Printf("cannot close peerConnection: %v\n", err)
		}
	}()

	// 设置处理ICE返回候选地址事件
	c.peerConn.OnICECandidate(c.OnICECandidate)
	c.peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {

	})

	// 发起创建连接到对端（Peer）的数据通道
	dataChannel, err := c.peerConn.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	dataChannel.OnOpen(func() {
		log.Println("data channel opened")
	})
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {

	})

	// 发送信令

}

// OnICECandidate 处理ICE返回候选地址事件
func (c *Client) OnICECandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}

	c.candidatesMux.Lock()
	defer c.candidatesMux.Unlock()

	log.Printf("OnICECandidate, candidate=%s\n", candidate.ToJSON().Candidate)
	// 获取当前Peer远程会话描述信息，包括配置信息、媒体流、编解码器、网络传输参数等
	//desc := c.peerConn.RemoteDescription()
	//if desc == nil {
	//    pendingCandidates = append(pendingCandidates, candidate)
	//} else if onICECandidateErr := signalCandidate(*answerAddr, candidate); onICECandidateErr != nil {
	//    panic(onICECandidateErr)
	//}
}
