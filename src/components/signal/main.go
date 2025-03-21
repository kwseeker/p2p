package main

import (
    "flag"
    "kwseeker.top/kwseeker/p2p/src/components/signal/server"
)

var signalServerAddr = flag.String("addr", ":18900", "http service address")

func main() {
    flag.Parse()
    server.NewServer(signalServerAddr).Run()
}
