package main

import (
    "fmt"
    "kwseeker.top/kwseeker/p2p/src/components/peer/client"
    "log"
    "os"
)

//var ssa = flag.String("ssa", ":18900", "signal server addr")

func main() {
    //flag.Parse()
    ssa := os.Getenv("SSA")
    isa := os.Getenv("ISA")
    if ssa == "" {
        ssa = ":18900"
    }
    if isa == "" {
        isa = "stun:stun.l.google.com:19302"
    }
    log.Printf("ssa: %s, isa: %s\n", ssa, isa)

    option := &client.Option{
        SignalServerAddr: ssa,
        SignalServerPath: "/signal",
        PingIntervalSec:  20,
        ICEServerAddr:    isa,
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
