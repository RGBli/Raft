package main

import (
	"log"
	"net/http"
	"net/rpc"
)

type VoteArgs struct {
	Term        int
	CandidateID int
}

type VoteReply struct {
	// current term, in case candidate can update its term
	Term int
	// true when candidate win the election
	VoteGranted bool
}

// RequestVote RPC method
func (rf *Raft) RequestVote(args VoteArgs, reply *VoteReply) error {
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return nil
	}

	if rf.votedFor == -1 {
		rf.currentTerm = args.Term
		rf.votedFor = args.CandidateID
		reply.Term = rf.currentTerm
		reply.VoteGranted = true
	}
	return nil
}

type AppendEntriesArgs struct {
	Term     int
	LeaderID int
	// previous log index
	PrevLogIndex int
	// PrevLogIndex term
	PrevLogTerm int
	// log entries to store, empty when representing heartbeat）
	Entries []LogEntry
	// committed index of Leader
	LeaderCommit int
}

type AppendEntriesReply struct {
	Success bool
	Term    int
	// if Follower index is less than Leader index, Leader will be inform of the next index to send
	NextIndex int
}

// Heartbeat RPC method
func (rf *Raft) Heartbeat(args AppendEntriesArgs, reply *AppendEntriesReply) error {
	// if leader term is less than current peer term
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return nil
	}

	rf.heartbeatChan <- struct{}{}
	// if no logs attached
	if len(args.Entries) == 0 {
		reply.Success = true
		reply.Term = rf.currentTerm
		return nil
	}

	// follower has been down once, so follower needs to tell leader its max index for next heartbeat
	if args.PrevLogIndex > rf.getLastIndex() {
		reply.Success = false
		reply.Term = rf.currentTerm
		reply.NextIndex = rf.getLastIndex() + 1
		return nil
	}

	// if logs attached, and everything goes well
	rf.logs = append(rf.logs, args.Entries...)
	rf.commitIndex = rf.getLastIndex()
	reply.Success = true
	reply.Term = rf.currentTerm
	reply.NextIndex = rf.getLastIndex() + 1
	return nil
}

// startRPC is used to start a RPC server
func (rf *Raft) startRPC(port string) {
	rpc.Register(rf)
	rpc.HandleHTTP()
	go func() {
		err := http.ListenAndServe(port, nil)
		if err != nil {
			log.Fatal("listen error: ", err)
		}
	}()
}