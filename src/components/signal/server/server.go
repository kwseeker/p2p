package server

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
)

var (
	SignalServer *Server // SignalServer 单例
	once         sync.Once
	counter      int32 // 历史连接数统计，同时作为客户端连接 ver 值来源，用于区分 cid 相同的连接
)

// Server 信令服务器
// 实现 Peer SDP信息 和 Candidate 候选地址的记录以及在 Peer 间转发
// 为实现双向和实时转发，使用 Socket 协议通信
type Server struct {
	addr        string
	connections map[string]*ClientConn // 客户端连接, cid -> ClientConn
	mu          sync.Mutex
}

func NewServer(addr *string) *Server {
	once.Do(func() {
		SignalServer = &Server{
			addr:        *addr,
			connections: make(map[string]*ClientConn),
		}
	})
	return SignalServer
}

// Run 信令服务器启动运行
func (s *Server) Run() {
	// WebSocket 连接一个路由每次都会新开一个连接，而实现 SDP Candidate 信息转发需要复用连接，
	// 所以需要在同一个路由中处理 SDP Candidate 信息转发, 不同的消息通过消息类型区分并分发处理
	http.HandleFunc("/signal", dispatchHandler)

	log.Printf("Signal server start at %s\n", s.addr)
	err := http.ListenAndServe(s.addr, nil)
	if err != nil {
		log.Fatalf("Signal server start failed at %s\n", s.addr)
		return
	}
}

func (s *Server) getConnection(cid string) (*ClientConn, bool) {
	//s.mu.Lock()
	//defer s.mu.Unlock()
	clientConn, ok := s.connections[cid]
	return clientConn, ok
}

func (s *Server) removeConnection(clientConn *ClientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	currentConn, ok := s.connections[clientConn.cid]
	if ok {
		err := clientConn.conn.Close()
		if err != nil {
			return
		}
		if currentConn.ver == clientConn.ver {
			delete(s.connections, clientConn.cid)
		}
		log.Printf("Removed client conn %s\n", clientConn.cid)
	}
}

// ClientConn 客户端连接信息
type ClientConn struct {
	cid      string // Peer A 要连接 Peer B 的话需要先通过 cid + authCode 校验
	authCode string
	conn     *websocket.Conn
	mu       sync.Mutex // 防止并发读写出现混乱
	ver      int32
}

func (c *ClientConn) checkAndWriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.conn.WriteJSON(v)
	if websocket.IsCloseError(err) {
		log.Println("Client conn closed!")
		SignalServer.removeConnection(c)
	}
	return err
}

var ugr = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

func dispatchHandler(w http.ResponseWriter, r *http.Request) {
	// 协议升级为 WebSocket
	conn, err := ugr.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	for {
		// 阻塞读取客户端消息
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		log.Printf("dispatchHandler received: %s", msg)
		// message 转成 Message
		message := Message{}
		if err := json.Unmarshal(msg, &message); err != nil {
			log.Println(err)
			return
		}

		switch message.Type {
		case TypeRegister:
			go registerPeerConn(msg, conn)
			break
		case TypeHeartbeat:
			handleHeartbeat()
			break
		case TypeSdp:
			handleSdp(msg)
			break
		case TypeCandidate:
			handleCandidate(msg)
			break
		default:
			log.Println("Unknown message type!")
		}
	}
}

// 上报 Peer 节点信息
func registerPeerConn(msg []byte, conn *websocket.Conn) {
	registerMessage := RegisterMessage{}
	if err := json.Unmarshal(msg, &registerMessage); err != nil {
		log.Println(err)
		return
	}

	// 记录Peer连接信息
	SignalServer.mu.Lock()
	cc, b := SignalServer.getConnection(registerMessage.Cid)
	// 先删除旧连接如果存在的话
	if b && cc != nil {
		SignalServer.removeConnection(cc)
	}
	clientConn := &ClientConn{
		cid:      registerMessage.Cid,
		authCode: registerMessage.AuthCode,
		conn:     conn,
		ver:      atomic.AddInt32(&counter, 1),
	}
	SignalServer.connections[registerMessage.Cid] = clientConn
	SignalServer.mu.Unlock()

	// 响应
	err := clientConn.checkAndWriteJSON(NewRegisterResponse(registerMessage, true))
	if err != nil {
		log.Println("Response register response failed!")
		return
	}
}

func handleHeartbeat() {
}

// 处理SDP信令, 解析信令内容，并转发给目标Peer
func handleSdp(msg []byte) {
	sdpMessage := SdpMessage{}
	if err := json.Unmarshal(msg, &sdpMessage); err != nil {
		log.Println(err)
		return
	}
	sd := sdpMessage.Sd
	log.Printf("signaling: type=%s, sdp=%s\n", sd.Type, sd.SDP)
	sdp, err := sd.Unmarshal()
	if err != nil {
		log.Println("SDP Unmarshal error:", err)
		return
	}

	// 校验参数中cid和authCode和目标peer实际的authCode
	clientConn, b := SignalServer.getConnection(sdpMessage.To)
	if !b {
		log.Println("Client conn not found!")
		return
	}
	if sdpMessage.AuthCode == clientConn.authCode {
		log.Println("AuthCode check failed!")
		return
	}
	// SDP 转发给目标 Peer, 暂时不管目标 Peer 是否处理成功 TODO
	clientConn.mu.Lock()
	err = clientConn.checkAndWriteJSON(sdp)
	if err != nil {
		log.Println("SDP relay failed!")
		return
	}
	clientConn.mu.Unlock()

	fromConn, b := SignalServer.getConnection(sdpMessage.From)
	if !b {
		log.Println("Client conn not found!")
		return
	}
	fromConn.mu.Lock()
	if err = fromConn.checkAndWriteJSON(NewSdpResponse(sdpMessage, true)); err != nil {
		log.Println("SDP relay response failed!")
		return
	}
	fromConn.mu.Unlock()
}

func handleCandidate(msg []byte) {
	candidateMessage := CandidateMessage{}
	if err := json.Unmarshal(msg, &candidateMessage); err != nil {
		log.Println(err)
		return
	}
	log.Printf("handleCandidate: From=%s, To=%s, candidate=%s\n",
		candidateMessage.From, candidateMessage.To, candidateMessage.Candidate)

	toConn, b := SignalServer.getConnection(candidateMessage.To)
	if !b {
		log.Println("Client conn not found!")
		return
	}

	toConn.mu.Lock()
	if err := toConn.checkAndWriteJSON(candidateMessage); err != nil {
		log.Println("Candidate relay failed!")
		return
	}
	toConn.mu.Unlock()

	fromConn, b := SignalServer.getConnection(candidateMessage.From)
	if !b {
		log.Println("Client conn not found!")
		return
	}
	fromConn.mu.Lock()
	if err := fromConn.checkAndWriteJSON(NewCandidateResponse(candidateMessage, true)); err != nil {
		log.Println("Candidate relay response failed!")
		return
	}
	fromConn.mu.Unlock()
}
