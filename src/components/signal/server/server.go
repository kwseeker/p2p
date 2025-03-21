package server

import (
    "encoding/json"
    "github.com/gorilla/websocket"
    "kwseeker.top/kwseeker/p2p/src/components/message"
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
        log.Fatalf("Signal server start failed at %s, err:%v\n", s.addr, err)
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
    log.Printf("checkAndWriteJSON: %v", v)
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
        // m 转成 MMeta
        m := message.MMeta{}
        if err := json.Unmarshal(msg, &m); err != nil {
            log.Println(err)
            break
        }

        switch m.Type {
        case message.TypeRegisterRequest:
            go registerPeerConn(msg, conn)
            break
        case message.TypeRegisterResponse:
            break
        //case message.TypeHeartbeatRequest:
        //    handleHeartbeat()
        //    break
        case message.TypeSdpRequest:
            handleSdp(msg)
            break
        case message.TypeSdpResponse:
            break
        case message.TypeCandidateRequest:
            handleCandidate(msg)
            break
        case message.TypeCandidateResponse:
            break
        default:
            log.Println("Unknown message type!")
        }
    }
}

// 上报 Peer 节点信息
func registerPeerConn(msg []byte, conn *websocket.Conn) {
    registerRequest := message.RegisterRequest{}
    if err := json.Unmarshal(msg, &registerRequest); err != nil {
        log.Println(err)
        return
    }

    // 记录Peer连接信息
    SignalServer.mu.Lock()
    cc, b := SignalServer.getConnection(registerRequest.Cid)
    // 先删除旧连接如果存在的话
    if b && cc != nil {
        SignalServer.removeConnection(cc)
    }
    clientConn := &ClientConn{
        cid:      registerRequest.Cid,
        authCode: registerRequest.AuthCode,
        conn:     conn,
        ver:      atomic.AddInt32(&counter, 1),
    }
    SignalServer.connections[registerRequest.Cid] = clientConn
    SignalServer.mu.Unlock()

    // 响应
    err := clientConn.checkAndWriteJSON(message.NewRegisterResponse(registerRequest, true))
    if err != nil {
        log.Printf("Response register response failed, err: %v\n", err)
        return
    }
}

//func handleHeartbeat() {
//}

// 处理SDP信令, 解析信令内容，并转发给目标Peer
func handleSdp(msg []byte) {
    sdpRequest := message.SdpRequest{}
    if err := json.Unmarshal(msg, &sdpRequest); err != nil {
        log.Println(err)
        return
    }
    log.Printf("handleSdp: From=%s, To=%s, sd=%s\n", sdpRequest.From, sdpRequest.To, sdpRequest.Sd)

    // 校验参数中cid和authCode和目标peer实际的authCode
    clientConn, b := SignalServer.getConnection(sdpRequest.To)
    if !b {
        log.Println("Client conn not found!")
        return
    }
    //if sdpRequest.AuthCode != clientConn.authCode {   //TODO
    //    log.Println("AuthCode check failed!")
    //    return
    //}
    // SDP 转发给目标 Peer, 暂时不管目标 Peer 是否处理成功 TODO
    err := clientConn.checkAndWriteJSON(sdpRequest)
    if err != nil {
        log.Println("SDP relay failed!")
        return
    }

    // 向来源端返回正常响应
    fromConn, b := SignalServer.getConnection(sdpRequest.From)
    if !b {
        log.Println("Client conn not found!")
        return
    }
    if err = fromConn.checkAndWriteJSON(message.NewSdpResponse(sdpRequest, true)); err != nil {
        log.Println("SDP relay response failed!")
        return
    }
}

func handleCandidate(msg []byte) {
    candidateRequest := message.CandidateRequest{}
    if err := json.Unmarshal(msg, &candidateRequest); err != nil {
        log.Println(err)
        return
    }
    log.Printf("handleCandidate: From=%s, To=%s, candidate=%s\n",
        candidateRequest.From, candidateRequest.To, candidateRequest.Candidate)

    toConn, b := SignalServer.getConnection(candidateRequest.To)
    if !b {
        log.Println("Client conn not found!")
        return
    }
    if err := toConn.checkAndWriteJSON(candidateRequest); err != nil {
        log.Println("Candidate relay failed!")
        return
    }

    fromConn, b := SignalServer.getConnection(candidateRequest.From)
    if !b {
        log.Println("Client conn not found!")
        return
    }
    if err := fromConn.checkAndWriteJSON(message.NewCandidateResponse(candidateRequest, true)); err != nil {
        log.Println("Candidate relay response failed!")
        return
    }
}
