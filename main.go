package main

import (
	"flag"
	"strings"
)

func main() {
	id := flag.Int("id", 1, "peer id")
	cluster := flag.String("cluster", "127.0.0.1:8080,127.0.0.1:8081", "seperated by comma")
	port := flag.String("port", ":8080", "rpc listen port")

	flag.Parse()
	clusters := strings.Split(*cluster, ",")

	peers := make(map[int]peer)
	for i, v := range clusters {
		peers[i] = peer(v)
	}

	raft := new(Raft)
	raft.id = *id
	raft.peers = peers
	raft.startRPC(*port)
	raft.startRaft()

	select {}
}
