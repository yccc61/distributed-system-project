//
// raft.go
// =======
// Write your code in this file
// We will use the original version of all other
// files for testing
//

package raft

//
// API
// ===
// This is an outline of the API that your raft implementation should
// expose.
//
// rf = NewPeer(...)
//   Create a new Raft server.
//
// rf.PutCommand(command interface{}) (index, term, isleader)
//   PutCommand agreement on a new log entry
//
// rf.GetState() (me, term, isLeader)
//   Ask a Raft peer for "me" (see line 58), its current term, and whether it thinks it
//   is a leader
//
// ApplyCommand
//   Each time a new entry is committed to the log, each Raft peer
//   should send an ApplyCommand to the service (e.g. tester) on the
//   same server, via the applyCh channel passed to NewPeer()
//

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/cmu440/rpc"
)

// Set to false to disable debug logs completely
// Make sure to set kEnableDebugLogs to false before submitting
const kEnableDebugLogs = false

// Set to true to log to stdout instead of file
const kLogToStdout = true

// Change this to output logs to a different directory
const kLogOutputDir = "./raftlogs/"

const Follower = 1
const Leader = 2
const Candidate = 3

// ApplyCommand
// ========
//
// As each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyCommand to the service (or
// tester) on the same server, via the applyCh passed to NewPeer()
type ApplyCommand struct {
	Index   int
	Command interface{}
}

type Logs struct {
	Term    int         //term when the entry was received by the leader
	Command interface{} //associated command
}

type ServerState int

// Raft struct
// ===========
//
// A Go object implementing a single Raft peer
type Raft struct {
	mux    sync.Mutex       // Lock to protect shared access to this peer's state
	peers  []*rpc.ClientEnd // RPC end points of all peers
	me     int              // this peer's index into peers[]
	logger *log.Logger      // We provide you with a separate logger per peer.

	currentTerm   int    //latest term server has seen (initialized to 0 on first boot, increases monotonically)
	votedFor      int    //candidateId that received vote in current term (or null if none)
	electionState int    //state of the server: follower, leader, or candidate
	log           []Logs //log entries; each entry contains command for state machine, and term when entry was received by leader (first index is 1)

	beLeader   chan bool               //send to mainroutine if  the candidate get enough votes
	receivedHb chan *AppendEntriesArgs //send to mainroutine if receive appendEntries
	lostElect  chan bool               //send to startelect if the candidate lost election
	lostAppend chan bool               // send to startAppendEntries if the server is no longer leader
	applyCh    chan ApplyCommand       //For commiting logs

	// volatile state on all servers

	commitIndex int //index of highest log entry known to be committed
	lastApplied int // index of highest log entry applied to state machine

	nextIndex  []int // for each server, index of next log entry to send to that server
	matchIndex []int // for each server, index of highest log entry know to be replicated on server

}

// GetState
// ==========
//
// Return "me", current term and whether this peer
// believes it is the leader
func (rf *Raft) GetState() (int, int, bool) {
	var me int
	var term int
	var isleader bool
	rf.mux.Lock()
	me = rf.me
	term = rf.currentTerm
	if rf.electionState == Leader {
		isleader = true
	} else {
		isleader = false
	}
	rf.mux.Unlock()
	return me, term, isleader
}

// RequestVoteArgs
// ===============
//
// Example RequestVote RPC arguments structure.
//
// # Please note: Field names must start with capital letters!
type RequestVoteArgs struct {
	Term         int // Candidate's term
	CandidateId  int // Which Candidate is requesting Vote?
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate's last log entry
}

// RequestVoteReply
// ================
//
// Example RequestVote RPC reply structure.
//
// # Please note: Field names must start with capital letters!
type RequestVoteReply struct {
	Term        int  // currentTerm from vote receiver, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

// RequestVote
// ===========
//
// Example RequestVote RPC handler
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mux.Lock()

	//If args.Term is less than currentTerm, do not grant vote
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		rf.mux.Unlock()
		return
	}
	//If term > currentTerm, currentTerm ← term (step down  if leader or candidate)
	//NOTE: Do not grant vote yet, only step down because log might be staled
	if args.Term > rf.currentTerm {
		reply.Term = rf.currentTerm
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.electionState = Follower
	}

	// If votedFor is null or candidateId, and candidate’s log is at
	// least as up-to-date as receiver’s log, grant vote
	// Case 1:Up to date meaning: candidate's last log term strictly larger than rf's last log term (case 1)
	// Case 2:Candidate's last log term is the same as rf's last log term, but contains more logs (case 2)
	case1 := args.LastLogTerm > rf.log[len(rf.log)-1].Term
	case2 := (args.LastLogTerm == rf.log[len(rf.log)-1].Term) && (args.LastLogIndex+1 >= len(rf.log))
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && (case1 || case2) {
		reply.VoteGranted = true
		reply.Term = rf.currentTerm
		if rf.votedFor == -1 {
			rf.votedFor = args.CandidateId
		}
	}
	rf.mux.Unlock()
}

// AppendEntriesArgs
// ===============
//
// Example AppendEntries RPC arguments structure.
//
// # Please note: Field names must start with capital letters!
type AppendEntriesArgs struct {
	Term         int    //leader’s term
	LeaderId     int    //leader’s Id so follower can redirect clients
	PrevLogIndex int    //index of log entry immediately preceding new ones
	PrevLogTerm  int    //term of prevLogIndex entry
	Entries      []Logs //log entries to store (empty for heartbeat;may send more than one for efficiency)
	LeaderCommit int    //leader’s commitIndex
}

// AppendEntriesReply
// ================
//
// AppendEntries RPC reply structure.
//
// # Please note: Field names must start with capital letters!
type AppendEntriesReply struct {
	Term    int  //term from the receiver
	Success bool //indicates if appendEntries was successful
}

// AppendEntries
// ===========
//
// AppendEntries RPC handler

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mux.Lock()
	defer rf.mux.Unlock()
	reply.Term = rf.currentTerm

	//Return if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}
	//If term > currentTerm, currentTerm ← term
	//If candidate or leader, step down
	rf.receivedHb <- args
	if args.Term > rf.currentTerm {
		reply.Success = false
		rf.currentTerm = args.Term
		rf.electionState = Follower
	}
	//Return failure if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	//NOTE: It is also possible that the length of the log is less than the prevLogIndex(edge case)
	if len(rf.log) <= args.PrevLogIndex || rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		return
	}

	//If existing entries conflict with new entries, find the starting index of inconsistency
	rfIdx := args.PrevLogIndex + 1
	argIdx := 0
	for argIdx < len(args.Entries) && rfIdx < len(rf.log) && (args.Entries[argIdx].Term == rf.log[rfIdx].Term) {
		argIdx += 1
		rfIdx += 1
	}
	rf.log = rf.log[:rfIdx]
	// Append any new entries not already in the log
	rf.log = append(rf.log, args.Entries[argIdx:]...)
	//Advance state machine with newly committed entries
	if rf.commitIndex < args.LeaderCommit {
		last := len(rf.log) - 1
		rf.commitIndex = min(last, args.LeaderCommit)
		go rf.applyCommit()
	}
	// reply successful in the end
	reply.Success = true

}

func (rf *Raft) applyCommit() {
	rf.mux.Lock()
	for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
		rf.applyCh <- ApplyCommand{Index: i, Command: rf.log[i].Command}
	}
	rf.lastApplied = rf.commitIndex
	rf.mux.Unlock()
}

// sendRequestVote
// ===============
//
// Example code to send a RequestVote RPC to a server.
//
// server int -- index of the target server in
// rf.peers[]
//
// args *RequestVoteArgs -- RPC arguments in args
//
// reply *RequestVoteReply -- RPC reply
//
// The types of args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers)
//
// The rpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost
//
// Call() sends a request and waits for a reply.
//
// If a reply arrives within a timeout interval, Call() returns true;
// otherwise Call() returns false
//
// Thus Call() may not return for a while.
//
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply
//
// Call() is guaranteed to return (perhaps after a delay)
// *except* if the handler function on the server side does not return
//
// Thus there
// is no need to implement your own timeouts around Call()
//
// Please look at the comments and documentation in ../rpc/rpc.go
// for more details
//
// If you are having trouble getting RPC to work, check that you have
// capitalized all field names in the struct passed over RPC, and
// that the caller passes the address of the reply struct with "&",
// not the struct itself
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// PutCommand
// =====
//
// The service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log
//
// If this server is not the leader, return false.
//
// Otherwise start the agreement and return immediately.
//
// There is no guarantee that this command will ever be committed to
// the Raft log, since the leader may fail or lose an election
//
// The first return value is the index that the command will appear at
// if it is ever committed
//
// The second return value is the current term.
//
// The third return value is true if this server believes it is
// the leader
func (rf *Raft) PutCommand(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true
	rf.mux.Lock()
	if rf.electionState != Leader {
		term = rf.currentTerm
		isLeader = false
		rf.mux.Unlock()
		return index, term, isLeader
	} else {
		//it is leader
		index = len(rf.log)
		term = rf.currentTerm
		rf.log = append(rf.log, Logs{term, command})
		rf.mux.Unlock()
		return index, term, isLeader
	}

}

// Stop
// ====
//
// The tester calls Stop() when a Raft instance will not
// be needed again
//
// You are not required to do anything
// in Stop(), but it might be convenient to (for example)
// turn off debug output from this instance
func (rf *Raft) Stop() {
}

// NewPeer
// ====
//
// The service or tester wants to create a Raft server.
//
// The port numbers of all the Raft servers (including this one)
// are in peers[]
//
// This server's port is peers[me]
//
// All the servers' peers[] arrays have the same order
//
// applyCh
// =======
//
// applyCh is a channel on which the tester or service expects
// Raft to send ApplyCommand messages
//
// NewPeer() must return quickly, so it should start Goroutines
// for any long-running work
func NewPeer(peers []*rpc.ClientEnd, me int, applyCh chan ApplyCommand) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.me = me

	if kEnableDebugLogs {
		peerName := peers[me].String()
		logPrefix := fmt.Sprintf("%s ", peerName)
		if kLogToStdout {
			rf.logger = log.New(os.Stdout, peerName, log.Lmicroseconds|log.Lshortfile)
		} else {
			err := os.MkdirAll(kLogOutputDir, os.ModePerm)
			if err != nil {
				panic(err.Error())
			}
			logOutputFile, err := os.OpenFile(fmt.Sprintf("%s/%s.txt", kLogOutputDir, logPrefix), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				panic(err.Error())
			}
			rf.logger = log.New(logOutputFile, logPrefix, log.Lmicroseconds|log.Lshortfile)
		}
		rf.logger.Println("logger initialized")
	} else {
		rf.logger = log.New(ioutil.Discard, "", 0)
	}
	//All server starts as follower, with 0 term and voted for nobody
	rf.electionState = Follower
	rf.currentTerm = 0
	rf.votedFor = -1

	//Channels for communication
	peerCount := len(rf.peers)
	rf.beLeader = make(chan bool)
	rf.receivedHb = make(chan *AppendEntriesArgs, peerCount)
	rf.lostElect = make(chan bool)
	rf.lostAppend = make(chan bool)

	//Defined in part b
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.log = make([]Logs, 1)
	rf.applyCh = applyCh
	go rf.mainRoutine()
	return rf
}

// Main Routine
func (rf *Raft) mainRoutine() {
	minimumTime := 300
	delta := 300
	duration := rand.Intn(delta) + minimumTime
	timerElect := time.NewTicker(time.Duration(duration) * time.Millisecond)
	for {
		select {
		case <-timerElect.C:
			timerElect.Stop()
			//when election timeout, start to elect
			go rf.startElect()
		case <-rf.receivedHb:
			timerElect.Stop()
			rf.mux.Lock()
			//if received heartbeat and still candidate-> step down to follower
			if rf.electionState == Candidate {
				rf.lostElect <- true
				rf.electionState = Follower
			}
			rf.mux.Unlock()

		case <-rf.beLeader:
			timerElect.Stop()
			rf.mux.Lock()
			//When becomes leader, start sending appendEntries
			total_peers := len(rf.peers)
			//nextIndex would be initialize to the length of logs from the leader
			rf.nextIndex = make([]int, total_peers)
			//When a server becomes a leader, it initializes all nextIndex values to the index just after the last one in its log
			total_log := len(rf.log)
			for i := range total_peers {
				rf.nextIndex[i] = total_log
			}
			rf.matchIndex = make([]int, total_peers)
			rf.electionState = Leader
			rf.mux.Unlock()
			rf.startAppendEntries()
		}
		timerElect.Reset(time.Duration(duration) * time.Millisecond)
	}
}

// The wrapper function send votes and process the reply
func (rf *Raft) sendRequestVoteChan(server int, args *RequestVoteArgs, reply *RequestVoteReply, replyChan chan *RequestVoteReply) {
	ok := rf.sendRequestVote(server, args, reply)
	if ok {
		replyChan <- reply
	}
}

func (rf *Raft) startElect() {
	rf.mux.Lock()
	rf.electionState = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf_currentTerm := rf.currentTerm
	rf_peerCount := len(rf.peers)
	rf_me := rf.me

	logIndex := 0
	logTerm := 0
	if len(rf.log) > 0 {
		logIndex = len(rf.log)
		logTerm = rf.log[len(rf.log)-1].Term
	}
	myArgs := &RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: logIndex,
		LastLogTerm:  logTerm,
	}
	rf.mux.Unlock()
	replyChan := make(chan *RequestVoteReply)
	for i := 0; i < len(rf.peers); i++ {
		if i == rf_me {
			continue
		}
		currReply := &RequestVoteReply{
			Term:        0,
			VoteGranted: false,
		}
		go rf.sendRequestVoteChan(i, myArgs, currReply, replyChan)
	}

	totalVotes := 1
	for {
		select {
		case newReply := <-replyChan:
			if newReply.Term > rf_currentTerm {
				rf.mux.Lock()
				rf.currentTerm = newReply.Term
				rf.electionState = Follower
				rf.votedFor = -1
				rf.mux.Unlock()
				return
			}
			if newReply.VoteGranted {
				totalVotes += 1
				//if get enough votes
				if totalVotes > rf_peerCount/2 {
					rf.mux.Lock()
					rf.beLeader <- true
					rf.mux.Unlock()
					return
				}
			}
		case <-rf.lostElect:
			return
		}

	}

}

func (rf *Raft) startAppendEntries() {
	for {
		select {
		case <-rf.lostAppend:
			rf.mux.Lock()
			rf.mux.Unlock()
			return
		default:
			rf.mux.Lock()
			rf_me := rf.me
			rf_peers := rf.peers
			if rf.electionState != Leader {
				rf.mux.Unlock()
				return
			}
			rf.mux.Unlock()
			//keep boardcasting heartbeat to other servers
			for i := 0; i < len(rf_peers); i++ {
				if i == rf_me {
					continue
				}
				//sending heartbeat: AppendEntries RPCs with no log entries
				rf.mux.Lock()
				prevLogTerm := 0

				prevLogIndex := 0
				if len(rf.log) > 0 && rf.nextIndex[i] > 0 {
					prevLogIndex = rf.nextIndex[i] - 1
					prevLogTerm = rf.log[rf.nextIndex[i]-1].Term
				}

				reply := &AppendEntriesReply{}
				args := &AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: prevLogIndex,    //-1 because nextIndex is initialize to the len of log
					PrevLogTerm:  prevLogTerm,     //Term of the log with prevlogIndex
					Entries:      make([]Logs, 0), //default as heartbeat with no entries
					LeaderCommit: rf.commitIndex,
				}
				isHeartbeat := true
				if len(rf.log)-1 >= rf.nextIndex[i] {
					isHeartbeat = false
					entriesStartIdx := rf.nextIndex[i]
					args.Entries = rf.log[entriesStartIdx:]
				}
				rf.mux.Unlock()
				go rf.sendAppendEntriesHelper(i, args, reply, isHeartbeat)
			}
			sleepTime := 100
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		}

	}

}

// The helper function send AppendEntries and process the replies
func (rf *Raft) sendAppendEntriesHelper(server int, args *AppendEntriesArgs, reply *AppendEntriesReply, isHeartbeat bool) {
	ok := rf.sendAppendEntries(server, args, reply)
	if ok {
		rf.processAppendEntriesReply(server, args, reply, isHeartbeat)
	}
}
func (rf *Raft) processAppendEntriesReply(server int, args *AppendEntriesArgs, reply *AppendEntriesReply, isHeartbeat bool) {
	//Process reply from appendEntries
	rf.mux.Lock()
	defer rf.mux.Unlock()
	if reply.Success {
		rf.nextIndex[server] = args.PrevLogIndex + len(args.Entries) + 1
		rf.matchIndex[server] = rf.nextIndex[server] - 1
	} else {
		//If AppendEntries fails because of log inconsistency,  decrement nextIndex and retry
		if rf.nextIndex[server] > 1 && !isHeartbeat {
			rf.nextIndex[server]--
		}
	}
	//For all not commited entries, commit when obtain the majority votes
	for entryIdx := rf.commitIndex + 1; entryIdx < len(rf.log); entryIdx += 1 {
		//Only log entries from the leader’s current term are committed by counting replicas
		if rf.log[entryIdx].Term != rf.currentTerm {
			continue
		}
		vote := 1
		for p := 0; p < len(rf.peers); p++ {
			if p == rf.me {
				continue
			} else {
				if rf.matchIndex[p] >= entryIdx {
					vote += 1
				}
			}
		}
		if vote > len(rf.peers)/2 {
			rf.commitIndex = entryIdx
			go rf.applyCommit()
			break
		}
	}

}
