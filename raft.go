package main

import (
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"
)

const (
	ElectionTimeout     = 150 * time.Millisecond
	HeartbeatInterval   = 100 * time.Millisecond
	LogGenerateInterval = 3000 * time.Millisecond
)

// State definition
type State int

// status of peer
const (
	Follower State = iota
	Candidate
	Leader
)

// LogEntry struct
type LogEntry struct {
	LogTerm  int
	LogIndex int
	LogCMD   interface{}
}

// peer is ip:port format string
type peer string

// Raft Node
type Raft struct {
	mu          sync.RWMutex
	id          int
	peers       map[int]peer
	state       State
	currentTerm int
	votedFor    int
	voteCount   int
	logs        []LogEntry
	// index of highest log entry known to be committed
	commitIndex int
	// index of highest log entry applied to state machine
	lastApplied int
	// next log index
	nextIndex []int
	// max index replicated to all the peers
	matchIndex    []int
	heartbeatChan chan struct{}
	toLeaderChan  chan struct{}
}

// broadcastRequestVote is called when a follower becomes a candidate
func (rf *Raft) broadcastRequestVote() {
	var args = VoteArgs{
		Term:        rf.currentTerm,
		CandidateID: rf.id,
	}

	for peer := range rf.peers {
		go func(i int) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("[broadcastRequestVote] err: %v", err)
				}
			}()
			var reply VoteReply
			rf.sendRequestVote(i, args, &reply)
		}(peer)
	}
}

func (rf *Raft) sendRequestVote(id int, args VoteArgs, reply *VoteReply) {
	client, err := rpc.DialHTTP("tcp", string(rf.peers[id]))
	if err != nil {
		log.Fatal("dialing: ", err)
	}

	defer client.Close()
	client.Call("Raft.RequestVote", args, reply)

	// if current candidate's term is not the newest
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.state = Follower
		rf.votedFor = -1
		return
	}

	if reply.VoteGranted {
		rf.voteCount++
	}

	if rf.voteCount >= len(rf.peers)/2+1 {
		rf.toLeaderChan <- struct{}{}
	}
}

// broadcastHeartbeat is called by leader to maintain the heartbeat
func (rf *Raft) broadcastHeartbeat() {
	for i := range rf.peers {
		var args AppendEntriesArgs
		args.Term = rf.currentTerm
		args.LeaderID = rf.id
		args.LeaderCommit = rf.commitIndex

		// calculate preLogIndex and preLogTerm
		// get entries and send to follower
		prevLogIndex := rf.nextIndex[i] - 1
		// if new logs are generated, they are attached in heartbeat
		if rf.getLastIndex() > prevLogIndex {
			args.PrevLogIndex = prevLogIndex
			args.PrevLogTerm = rf.logs[prevLogIndex].LogTerm
			args.Entries = rf.logs[prevLogIndex:]
			log.Printf("leader-%d send entries to %d: %+v\n", rf.id, i, args.Entries)
		}

		go func(i int, args AppendEntriesArgs) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("[broadcastHeartbeat] err: %v", err)
				}
			}()
			var reply AppendEntriesReply
			rf.sendHeartbeat(i, args, &reply)
		}(i, args)
	}
}

func (rf *Raft) sendHeartbeat(id int, args AppendEntriesArgs, reply *AppendEntriesReply) {
	client, err := rpc.DialHTTP("tcp", string(rf.peers[id]))
	if err != nil {
		log.Fatal("dialing:", err)
	}

	defer client.Close()
	client.Call("Raft.Heartbeat", args, reply)

	if reply.Success {
		if reply.NextIndex > 0 {
			rf.nextIndex[id] = reply.NextIndex
			rf.matchIndex[id] = reply.NextIndex - 1
		}
		return
	}
	// if leader falls behind the follower, leader becomes follower
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.state = Follower
		rf.votedFor = -1
	} else { // if follower was down once
		rf.nextIndex[id] = reply.NextIndex
		rf.matchIndex[id] = reply.NextIndex - 1
	}
}

// getLastIndex returns the last index of log entries
func (rf *Raft) getLastIndex() int {
	logLen := len(rf.logs)
	if logLen == 0 {
		return 0
	}
	return rf.logs[logLen-1].LogIndex
}

// startRaft is used to start a raft node
func (rf *Raft) startRaft() {
	// raft peer is initialized as a follower
	rf.state = Follower
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.heartbeatChan = make(chan struct{})
	rf.toLeaderChan = make(chan struct{})

	for {
		switch rf.state {
		case Follower:
			select {
			case <-rf.heartbeatChan:
				log.Printf("follower-%d recived heartbeat\n", rf.id)
			// candidate election timeout and follower heartbeat timeout both set as a random duration of 150-300ms for convenience
			case <-time.After(randDuration(ElectionTimeout, ElectionTimeout*2)):
				log.Printf("follower-%d timeout\n", rf.id)
				rf.state = Candidate
			}
		case Candidate:
			log.Printf("follower-%d becomes candidate", rf.id)
			rf.currentTerm++
			rf.votedFor = rf.id
			rf.voteCount = 1
			go func() {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("[broadcastHeartbeat] err: %v", err)
					}
				}()
				rf.broadcastRequestVote()
			}()

			select {
			// fail to become leader after election timeout
			case <-time.After(randDuration(ElectionTimeout, ElectionTimeout*2)):
				rf.state = Follower
			case <-rf.toLeaderChan:
				log.Printf("candidate-%d becomes leader", rf.id)
				rf.state = Leader

				// initialize peers' nextIndex and matchIndex
				rf.nextIndex = make([]int, len(rf.peers))
				rf.matchIndex = make([]int, len(rf.peers))
				for i := range rf.peers {
					rf.nextIndex[i] = 1
					rf.matchIndex[i] = 0
				}

				// start to generate logs
				go func() {
					defer func() {
						if err := recover(); err != nil {
							log.Printf("[startRaft] err: %v", err)
						}
					}()
					for i := 1; ; i++ {
						rf.logs = append(rf.logs, LogEntry{rf.currentTerm, i, fmt.Sprintf("send to %d peers", len(rf.peers))})
						time.Sleep(LogGenerateInterval)
					}
				}()
			}
		case Leader:
			rf.broadcastHeartbeat()
			// leader broadcast heartbeat every 100ms
			time.Sleep(HeartbeatInterval)
		}
	}
}
