# 测试环境部署

使用 Docker 虚拟网络模拟 WebRTC 跨公网通信环境。

**3种组件**：

+ 信令服务器

  Peer 之间使用 SDP 协议（其实是一种编码格式）通过信令服务器交换信息，常采用 WebSocket 传输 SDP 数据。

  信令服务器也有一些开源的方案，也可以自行实现，比如借助 、Socket.IO 开发自己的信令服务器。

+ NAT STUN/TURN 服务器

  本地检测发现我的电脑属于对称型NAT，使用 Google、QQ、MIWIFI 等公网 STUN 服务，无法建立P2P，也无法通信。

  公网 TURN 中继服务器一般又是收费的，所以推荐自己搭建。

  可使用 Coturn 搭建 STUN/TURN 服务器。

  ```shell
  docker pull docker.1ms.run/coturn/coturn:4.6.2-alpine
  # https://1ms.run/r/coturn/coturn
  # 这里将宿主机网络作为公网
  # 和宿主机共享网络
  # STUN 默认端口 3478 TURN 默认端口5349
  # STUN/TURN 出口端口映射范围 49160-49170
  docker run --name coturn -d \
     --network=host \
     coturn/coturn:4.6.2-alpine --min-port=49160 --max-port=49170 \
     --external-ip=192.168.8.100 \
     --relay-ip=192.168.8.100
  ```

+ Peer

**网络模拟**：

+ 本地网络模拟公网

+ Docker 创建两个桥接网络模拟内部网络

  桥接网络中的容器默认也是通过主机NAT访问外部网络的。
  
  另外 Docker 网络默认也是对称型 NAT,  不过可以通过修改 iptables 设置 Docker 容器的网络的所有主站请求映射到固定端口，从而实现非对称 NAT。比如：
  
  ```shell
  # 将来自 Docker 网络（172.18.0.0/16）的 TCP 数据包的源地址和端口转换为当前主机的 IP 地址和端口，从而实现网络地址转换
  sudo iptables -t nat -A POSTROUTING -s 172.25.0.0/24 -p tcp -j SNAT --to-source :9025
  sudo iptables -t nat -A POSTROUTING -s 172.25.0.0/24 -p udp -j SNAT --to-source :9025
  
  sudo iptables -t nat -A POSTROUTING -s 172.26.0.0/24 -p tcp -j SNAT --to-source :9026
  sudo iptables -t nat -A POSTROUTING -s 172.26.0.0/24 -p udp -j SNAT --to-source :9026TODO:
  ```
  
  TODO：
  
  测试发现ICE候选地址检测无法获取“模拟公网”的候选地址。可能这个“模拟公网”地址不是真正的公网，STUN服务器无法识别？难道不能这么用？可能需要看 STUN 服务器获取候选地址的原理。

**目标**：

+ 实现两个虚拟网络的 Peer 通信

