package client

import (
    "encoding/json"
    "github.com/gorilla/websocket"
    "github.com/pion/webrtc/v4"
    "kwseeker.top/kwseeker/p2p/src/components/message"
    "log"
    "net/url"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"
)

const (
    PeerTypeOffer = iota
    PeerTypeAnswer
)

type Option struct {
    SignalServerAddr string // 信令服务器地址
    SignalServerPath string
    PingIntervalSec  int
    ICEServerAddr    string // ICE服务器地址
    PeerType         int    // Peer类型
    Cid              string // 客户端ID
    AuthCode         string // 认证码
}

type SignalServerConfig struct {
    SignalServerAddr string
    SignalServerPath string
    pingInterval     time.Duration
}

type ICEServerConfig struct {
    ICEServerAddr string
}

// Client Peer 节点在P2P连接建立前只会和信令服务器和ICE服务器进行通信
type Client struct {
    signalServerConfig SignalServerConfig
    iceServerConfig    ICEServerConfig
    peerType           int
    cid                string                 // 客户端ID
    authCode           string                 // 认证码
    toCid              *string                // 对端设备ID
    toAuthCode         *string                // 对端设备认证码
    peerConn           *webrtc.PeerConnection // 与ICE服务器的连接 PeerConnection
    pendingCandidates  []*webrtc.ICECandidate // 可能ICE服务器在Offer端发起对等连接前返回了一些候选地址,需要暂存起来用于后续通过SDP发给对端
    signalConn         *websocket.Conn        // 与信令服务器的WebSocket连接
    dataChannel        *webrtc.DataChannel    // 与对端Peer的数据通道
    wChan              chan bool              // DataChannel 是否写就绪
    candidatesMux      sync.Mutex
}

func NewClient(option *Option) *Client {
    return &Client{
        signalServerConfig: SignalServerConfig{
            SignalServerAddr: option.SignalServerAddr,
            SignalServerPath: option.SignalServerPath,
            pingInterval:     time.Duration(option.PingIntervalSec) * time.Second,
        },
        iceServerConfig: ICEServerConfig{
            ICEServerAddr: option.ICEServerAddr,
        },
        peerType: option.PeerType,
        cid:      option.Cid,
        authCode: option.AuthCode,
        wChan:    make(chan bool),
    }
}

func (c *Client) RunAsAnswer() {
    c.run(nil, nil)
}

func (c *Client) RunAsOffer(toCid *string, toAuthCode *string) {
    c.run(toCid, toAuthCode)
}

// run Peer节点启动
func (c *Client) run(toCid *string, toAuthCode *string) {
    c.toCid = toCid
    c.toAuthCode = toAuthCode

    // 1 连接信令服务器并上报本端信息
    c.connectSignalServer()

    // 2 连接ICE服务器
    config := webrtc.Configuration{
        ICEServers: []webrtc.ICEServer{
            {
                URLs: []string{c.iceServerConfig.ICEServerAddr},
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
    c.peerConn.OnICECandidate(c.onICECandidate)
    // 设置处理ICE连接状态变化事件
    c.peerConn.OnConnectionStateChange(c.onConnectionStateChange)

    if c.peerType == PeerTypeAnswer {
        // 设置成功建立 DataChannel 的监听
        c.peerConn.OnDataChannel(c.onDataChannel)
    } else if c.peerType == PeerTypeOffer {
        // 发起创建连接到对端（Peer）的数据通道
        c.dataChannel, err = c.peerConn.CreateDataChannel("data", nil)
        if err != nil {
            log.Printf("create DataChannel failed: %v\n", err)
            return
        }
        c.dataChannel.OnOpen(c.onOpen)
        c.dataChannel.OnMessage(c.onMessage)
    }

    // 3 Offer Peer 发起对等连接
    if c.peerType == PeerTypeOffer {
        // 创建 offer sdp
        offerSd, err := c.peerConn.CreateOffer(nil)
        if err != nil {
            log.Printf("CreateOffer failed: %v", err)
            return
        }
        // 设置本端描述信息
        if err := c.peerConn.SetLocalDescription(offerSd); err != nil {
            log.Printf("SetLocalDescription failed: %v", err)
            return
        }
        offerSdp := message.NewSdpRequest(offerSd, c.cid, *c.toCid, *c.toAuthCode)
        if err := c.signalConn.WriteJSON(offerSdp); err != nil {
            log.Printf("register local peer info to signal server, err: %v\n", err)
            return
        }
    }

    c.listenForShutdown()
}

func (c *Client) connectSignalServer() {
    u := url.URL{Scheme: "ws", Host: c.signalServerConfig.SignalServerAddr, Path: c.signalServerConfig.SignalServerPath}
    log.Printf("connecting to signal server %s", u.String())
    var err error
    if c.signalConn, _, err = websocket.DefaultDialer.Dial(u.String(), nil); err != nil {
        log.Fatalf("connecting to signal server, err: %v\n", err)
    }

    // 上报本端信息到信令服务器
    if err := c.signalConn.WriteJSON(message.NewRegisterRequest(c.cid, c.authCode)); err != nil {
        c.signalConn.Close()
        log.Fatalf("register local peer info to signal server, err: %v\n", err)
    }
    log.Printf("register peer info to signal server, cid=%s, authCode=%s", c.cid, c.authCode)

    // 监听信令服务器返回的消息，SDP、Candidate
    go func() {
        for {
            _, msg, err := c.signalConn.ReadMessage()
            if err != nil {
                log.Printf("read message from signal server failed: %v", err)
                break
            }

            log.Printf("received message from signal server: %s", msg)
            // message 转成 MMeta
            m := message.MMeta{}
            if err := json.Unmarshal(msg, &m); err != nil {
                log.Println(err)
                break
            }

            switch m.Type {
            case message.TypeRegisterResponse:
                //暂时忽略
                break
            case message.TypeSdpRequest:
                // 收到对端经过信令服务器中转的 SDP 消息
                sdpMessage := message.SdpRequest{}
                if err := json.Unmarshal(msg, &sdpMessage); err != nil {
                    log.Printf("unmarshal sdpMessage failed: %v", err)
                    break
                }
                // 本地 peerConnection 记录远端的描述信息
                if err := c.peerConn.SetRemoteDescription(sdpMessage.Sd); err != nil {
                    log.Printf("SetRemoteDescription failed: %v", err)
                    break
                }
                if c.peerType == PeerTypeAnswer {
                    c.toCid = &sdpMessage.From
                    // 1 作为 Answer 端还需要创建 answer sdp 并经过信令服务器转发给 offer端
                    answer, err := c.peerConn.CreateAnswer(nil)
                    if err != nil {
                        log.Printf("CreateAnswer failed: %v", err)
                        break
                    }
                    err = c.peerConn.SetLocalDescription(answer)
                    if err != nil {
                        panic(err)
                    }
                    answerSdp := message.NewSdpRequest(answer, c.cid, sdpMessage.From, "")
                    if err := c.signalConn.WriteJSON(answerSdp); err != nil {
                        log.Printf("register local peer info to signal server, err: %v\n", err)
                        break
                    }
                }
                // 2 通过信令服务器向对端发送ICE Candidate 信息
                for _, candidate := range c.pendingCandidates {
                    if err := c.signalCandidate(candidate); err != nil {
                        log.Printf("signalCandidate failed: %v", err)
                        break
                    }
                }
                break
            case message.TypeSdpResponse:
                // 暂时忽略
                break
            case message.TypeCandidateRequest:
                // 收到对端的候选地址信息后，记录到 peerConnection
                candidateMessage := message.CandidateRequest{}
                if err := json.Unmarshal(msg, &candidateMessage); err != nil {
                    log.Printf("unmarshal sdpMessage failed: %v", err)
                    break
                }
                if err := c.peerConn.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidateMessage.Candidate}); err != nil {
                    log.Printf("AddICECandidate failed: %v", err)
                    break
                }
                break
            case message.TypeCandidateResponse:
                // 暂时忽略
                break
            default:
                log.Println("Unknown message type!")
            }
        }
    }()

    // 维持 signalConn 连接
    go func() {
        ticker := time.NewTicker(c.signalServerConfig.pingInterval)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                if err := c.signalConn.WriteMessage(websocket.PingMessage, nil); err != nil {
                    log.Printf("signalConn write ping message error: %v", err)
                    return
                }
            }
        }
    }()
}

// OnICECandidate 处理ICE返回候选地址事件
func (c *Client) onICECandidate(candidate *webrtc.ICECandidate) {
    if candidate == nil {
        return
    }

    c.candidatesMux.Lock()
    defer c.candidatesMux.Unlock()

    log.Printf("onICECandidate, candidate=%s\n", candidate.ToJSON().Candidate)
    // 将从ICE服务器获取当前Peer远程会话描述信息，包括配置信息、媒体流、编解码器、网络传输参数等，通过信令服务器转发给对端
    desc := c.peerConn.RemoteDescription() // 估计是 SetRemoteDescription() 方法设置的, 即收到对端 SDP 消息后才能直接发送
    if desc == nil {                       // 说明还没有与对端建立通信
        c.pendingCandidates = append(c.pendingCandidates, candidate)
    } else if err := c.signalCandidate(candidate); err != nil {
        log.Printf("onICECandidate, signalCandidate err: %s\n", err)
    }
}

func (c *Client) listenForShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-sigChan:
        log.Printf("Received signal: %v. Shutting down...\n", sig)
        //c.Close()
        //os.Exit(0)
    }
}

// signalCandidate 发送ICE候选地址到对端，通过信令服务器转发, TODO 批量发送
func (c *Client) signalCandidate(candidate *webrtc.ICECandidate) error {
    // 与信令服务器的WebSocket连接是否存在，不存在则创建
    //if c.signalConn == nil {
    //}

    // 发送ICE候选地址到信令服务器
    candidateMessage := message.NewCandidateRequest(candidate.ToJSON().Candidate, c.cid, *c.toCid)
    if err := c.signalConn.WriteJSON(candidateMessage); err != nil {
        log.Printf("signalCandidate, candiatemessage: %v, err: %v\n", candidateMessage, err)
        return err
    }
    return nil
}

func (c *Client) onConnectionStateChange(state webrtc.PeerConnectionState) {
    log.Printf("Peer Connection State has changed: %s\n", state.String())

    if state == webrtc.PeerConnectionStateFailed {
        // Wait until PeerConnection has had no network activity for 30 seconds or another failure.
        // It may be reconnected using an ICE Restart.
        // Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
        // Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
        log.Println("Peer Connection has gone to failed exiting")
        c.Close()
        os.Exit(0)
    }

    if state == webrtc.PeerConnectionStateClosed {
        // PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
        log.Println("Peer Connection has gone to closed exiting")
        c.Close()
        os.Exit(0)
    }
}

func (c *Client) onDataChannel(dataChannel *webrtc.DataChannel) {
    log.Printf("New DataChannel establisted, label=%s, id=%d\n", dataChannel.Label(), dataChannel.ID())
    c.dataChannel = dataChannel

    dataChannel.OnOpen(c.onOpen)
    dataChannel.OnMessage(c.onMessage)
}

func (c *Client) onOpen() {
    log.Printf("DataChannel '%s'-'%d' open\n", c.dataChannel.Label(), c.dataChannel.ID())
    // 写就绪
    c.wChan <- true
    //ticker := time.NewTicker(5 * time.Second)
    //defer ticker.Stop()
    //for range ticker.C {
    //    msg, sendTextErr := randutil.GenerateCryptoRandomString(15, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
    //    if sendTextErr != nil {
    //        log.Printf("generate crypto random string, err: %v\n", sendTextErr)
    //    }
    //
    //    log.Printf("Sending '%s'\n", msg)
    //    if sendTextErr = c.dataChannel.SendText(msg); sendTextErr != nil {
    //        panic(sendTextErr)
    //    }
    //}
}

func (c *Client) WaitWritable() {
    writable := <-c.wChan
    if writable {
        log.Println("DataChannel now is writable -> ")
    }
}

func (c *Client) WriteText(text string) {
    if err := c.dataChannel.SendText(text); err != nil {
        log.Printf("send text:%s, error: %v\n", text, err)
    }
}

func (c *Client) onMessage(msg webrtc.DataChannelMessage) {
    log.Printf("MMeta from DataChannel '%s': '%s'\n", c.dataChannel.Label(), string(msg.Data))
}

func (c *Client) Close() {
    if c.peerConn != nil {
        if err := c.peerConn.Close(); err != nil {
            log.Printf("close peerConnection error: %v\n", err)
            return
        }
    }
    // TODO 清理信令服务器中的客户端连接信息
    if c.signalConn != nil {
        if err := c.signalConn.Close(); err != nil {
            log.Printf("close signalConn error: %v\n", err)
            return
        }
    }
}
