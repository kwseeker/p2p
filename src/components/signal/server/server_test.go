package server

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestPeerConnectServer(t *testing.T) {
	//t.SkipNow()

	addr := "localhost:18901"
	// 启动服务端
	go func() {
		NewServer(&addr).Run()
	}()

	time.Sleep(100 * time.Millisecond)
	u := url.URL{Scheme: "ws", Host: addr, Path: "/signal"}
	log.Printf("connecting to %s", u.String())
	// 启动两个Peer C1
	c1, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c1.Close()
	// C2
	c2, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c2.Close()

	err = c1.WriteJSON(NewRegisterMessage("c1", "c1code"))
	if err != nil {
		return
	}
	err = c1.WriteJSON(NewRegisterMessage("c2", "c2code"))
	if err != nil {
		return
	}

	go func() {
		for {
			log.Println("c1 listen")
			_, message, err := c1.ReadMessage()
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			log.Printf("c1 received: %s", message)
		}
	}()

	go func() {
		for {
			log.Println("c2 listen")
			_, message, err := c2.ReadMessage()
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			log.Printf("c2 received: %s", message)
		}
	}()

	select {}
}

// WebSocket 连接每一个路由都是一个新的连接，每个连接都需要额外进行协议升级
// 所有此处场景，如果想要使用 WebSocket 复用连接，转发 SDP 和 Candidate 信息需要通过同一个路由实现，
// 可以在这个路由内部，自行实现不同命令分发处理
func TestWebSocketCS(t *testing.T) {
	go func() {
		http.HandleFunc("/echo", echoHandler)   // 注册 /echo 路由
		http.HandleFunc("/hello", helloHandler) // 注册 /hello 路由
		log.Println("WebSocket server is running on ws://localhost:8080/echo")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal("Server error:", err)
		}
	}()

	// 等待1s
	time.Sleep(1 * time.Second)

	addr := "localhost:8080"
	u := url.URL{Scheme: "ws", Host: addr, Path: "/echo"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	// 设置读取超时
	if err := c.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
		return
	}

	// 创建登录消息
	RegisterMessage := NewRegisterMessage("testCid", "testAuthCode")
	loginData, err := json.Marshal(RegisterMessage)
	if err != nil {
		t.Fatalf("marshal login message: %v", err)
	}
	for i := 0; i < 3; i++ {
		// 发送登录消息
		err = c.WriteMessage(websocket.TextMessage, loginData)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		// 读取响应
		_, message, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		log.Printf("recv: %s", message)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		// 读取客户端发送的消息
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		log.Printf("Received: %s", message)

		// 将消息原样返回给客户端
		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Println("Write error:", err)
			break
		}
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	// 向客户端发送欢迎消息
	welcomeMessage := "Hello! Welcome to the WebSocket server."
	if err := conn.WriteMessage(websocket.TextMessage, []byte(welcomeMessage)); err != nil {
		log.Println("Write error:", err)
		return
	}
	log.Println("Sent welcome message to client on /hello")
}
