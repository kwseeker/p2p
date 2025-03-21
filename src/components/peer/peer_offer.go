package main

import (
    "fmt"
    "kwseeker.top/kwseeker/p2p/src/components/peer/client"
)

func main() {
    option := &client.Option{
        SignalServerAddr: "127.0.0.1:18900",
        SignalServerPath: "/signal",
        PingIntervalSec:  20,
        ICEServerAddr:    "stun:stun.l.google.com:19302",
        PeerType:         client.PeerTypeOffer,
        Cid:              "345 822 666", // 比如远程控制程序，每个终端都有一个设备码
        AuthCode:         "666999",      // 临时密码
    }

    toCid := "345 822 232"
    toAuthCode := "123456"
    answerPeer := client.NewClient(option)
    go answerPeer.RunAsOffer(&toCid, &toAuthCode)

    // 等待 DataChannel 继续
    answerPeer.WaitWritable()
    answerPeer.WriteText("Hello, I am AnswerPeer, cid=" + option.Cid)
    var text string
    for {
        // 读取终端输入
        _, _ = fmt.Scan(&text)
        answerPeer.WriteText(option.Cid + " >>> " + text)
    }
}
