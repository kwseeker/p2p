# 环境部署

WebRTC 跨公网通信模拟环境部署。

**3种组件**：

+ 信令服务器

  Peer 之间使用 SDP 协议（其实是一种编码格式）通过信令服务器交换信息，常采用 WebSocket 传输 SDP 数据。

  信令服务器也有一些开源的方案，也可以自行实现，比如借助 Socket.IO 开发自己的信令服务器。

  Socket.IO Go 实现：[go-socket.io](https://github.com/googollee/go-socket.io)。

+ NAT穿透服务器

  使用 Coturn 搭建 STUN/TURN 服务器。

+ Peer

**网络模拟**：

+ 本地网络模拟公网

+ Docker 创建两个桥接网络模拟内部网络

  桥接网络中的容器默认也是通过主机NAT访问外部网络的。

**目标**：

+ 实现两个虚拟网络的 Peer 通信

