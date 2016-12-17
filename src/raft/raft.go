package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, Term, isleader)
//   start agreement on a new log entry
// rf.GetState() (Term, isLeader)
//   ask a Raft for its current Term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import (
	"labrpc"
	"time"
	"math/rand"
)

// import "bytes"
// import "encoding/gob"


//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

type LogEntry struct {
	LogIndex int
	LogTerm  int
	LogCmd   interface{}
}

const (
	STATE_LEADER = 1
	STATE_CANDIDATE = 2
	STATE_FOLLOWER = 3
	HBINTERVAL = 100 * time.Millisecond
)
//

// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu          sync.Mutex
	peers       []*labrpc.ClientEnd
	persister   *Persister
	me          int             // index into peers[]

				    // Your data here.
				    // Look at the paper's Figure 2 for a description of what
				    // state a Raft server must maintain.

	stat          int
	currentTerm   int           // 服务器最后一次知道的任期号
	voteFor       int           // 当前获得选票的候选人id
	log           [] LogEntry   // 日志

				    //领导人需要使用的状态
	commitIndex   int           //已知的最大的已经被提交的日志条目的索引值
	lastApplied   int           //最后被应用到状态机的日志条目索引值
	nextIndex     [] int        //每个服务器需要给他发送的下一个日志条目索引
	matchIndex    [] int        //每个服务器已经复制给他的日志最高索引

	heartbeatChan chan bool     // 从其他服务器收到心跳
	requestVoteChan chan bool   // 从其他服务器收到发起投票的请求
	applyCh       chan ApplyMsg //client发送消息的chan
				    //候选人需要的状态
	winCount      int           //得到了多少选票
	winChan       chan bool     //赢得选举则给这个ch发一个消息
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	var term int
	var isleader bool
	// Your code here.
	term = rf.currentTerm
	isleader = rf.isLeader()

	return term, isleader
}

func (rf *Raft) isLeader() (bool) {
	return rf.stat == STATE_LEADER
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here.
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
}



//
// example RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
			 // Your data here.
	Term         int //候选人的任期号
	CandidatedId int //请求选票的候选人的id
	LastLogIndex int //候选人的最后日志条目的索引值
	LastLogTerm  int //候选人最后日志条目的任期号
}

//
// example RequestVote RPC reply structure.
//
type RequestVoteReply struct {
			 // Your data here.
	Term        int  //当前任期号
	VoteGranted bool //候选人是否赢得该选票
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here.
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.requestVoteChan <- true
	reply.VoteGranted = false
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		DPrintf("1")
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.voteFor = -1
		rf.stat = STATE_FOLLOWER
		rf.winCount = 0
	}
	isUptoDate := false

	if args.LastLogTerm > rf.getLastTerm() {
		isUptoDate = true
	} else if args.LastLogTerm == rf.getLastTerm() && args.LastLogIndex >= rf.getLastIndex() {
		isUptoDate = true
	}

	if isUptoDate && rf.voteFor == -1 {
		DPrintf("%d win %d's vote",args.CandidatedId ,rf.me)
		rf.voteFor = args.CandidatedId
		reply.VoteGranted = true
		reply.Term = rf.currentTerm
	}
	return
}


//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	DPrintf("%d send requestvote to %d", rf.me, server)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	//todo 这里要判断是不是已经变成leader了
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if !ok {
		return ok
	}
	if args.Term != reply.Term {
		return ok
	}
	if rf.currentTerm < reply.Term {
		rf.currentTerm = reply.Term
		rf.stat = STATE_FOLLOWER
		rf.voteFor = -1
		rf.winCount = 0
		return ok
	}
	if rf.stat == STATE_CANDIDATE && reply.VoteGranted {
		rf.winCount++
		if rf.winCount > len(rf.peers)/2 {
			DPrintf("%d win the selected", rf.me)
			rf.winChan <- true
			rf.stat = STATE_FOLLOWER
		}
	}

	return ok
}



func (rf *Raft) broadcastRequestVote() {
	// 如果失去心跳,调用这个命令来搞事情
	// 通过go 异步发送sendrequestvote, 并在sendrequestvote中处理结果
	rf.mu.Lock()
	var rvg RequestVoteArgs
	rvg.Term = rf.currentTerm
	rvg.CandidatedId = rf.me
	rvg.LastLogTerm = rf.getLastTerm()
	rvg.LastLogIndex = rf.getLastIndex()
	rf.mu.Unlock()
	DPrintf("%d broadcastRequestVote to all", rf.me)
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		go func(i int) {
			var reply RequestVoteReply
			rf.sendRequestVote(i, rvg, &reply)
		}(i)
	}
}





func (rf *Raft) broadcastAppendEntries() {

}



func (rf *Raft) ReceiveHeartBeat() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.heartbeatChan <- true
	//收到心跳, 处理事情。包括:
	// 1. 收到权威心跳,自己变为跟随者
	// 2. 收到需要添加的log,添加他

}
//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// Term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	DPrintf("starat raft")
	rf.mu.Lock()
	defer rf.mu.Unlock()
	index := -1
	term := rf.currentTerm
	isLeader := (rf.stat == STATE_LEADER)
	if (isLeader) {
		//todo append log entry. 需要新的broadcastAppend函数
	}

	return index, term, isLeader
}



//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here.

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	// 上来先设置自己为跟随者,如果没有收到心跳,则发起选票
	// 跟随者,timeout则变成候选人,如果收到心跳则重新计时。利用select 重新chan计时
	// 领导者,监听applych,发送状态。
	// 候选人,阻塞直到timeout或者收到vote。收到vote的时候应该是reply里面的voteFor是自己,这里也用chan实现。
	// 当然收到权威声明则变成候选人
	rf.stat = STATE_FOLLOWER
	rf.currentTerm = 0
	rf.voteFor = -1
	rf.log = append(rf.log, LogEntry{LogTerm: 0})

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.heartbeatChan = make(chan bool, 100)
	rf.requestVoteChan = make(chan bool, 100)
	rf.winChan = make(chan bool, 100)

	rf.applyCh = applyCh
	DPrintf("rf %d, state %d", rf.me, rf.stat)
	go func() {
		for {
			switch rf.stat {
				case STATE_FOLLOWER:{
					DPrintf("%d stat is to follower", rf.me)
					select {
					case <-rf.heartbeatChan:
					case <-rf.requestVoteChan:
					case <-time.After(time.Duration(rand.Int63() % 333 + 550) * time.Millisecond):
						rf.mu.Lock()
						rf.stat = STATE_CANDIDATE
						rf.mu.Unlock()
					}
				}
				case STATE_LEADER: {
					DPrintf("%d stat is to leader",rf.me)

					//这个函数第一次还负责发送权威声明
					//rf.broadcastAppendEntries()
					time.Sleep(HBINTERVAL)

				}
				case STATE_CANDIDATE:{
					DPrintf("%d stat is to candidate",rf.me)
					rf.mu.Lock()
					rf.currentTerm ++
					rf.voteFor = rf.me
					rf.winCount = 1
					rf.mu.Unlock()
					rf.broadcastRequestVote()
					select {
					case <- rf.winChan:
					//成为leader
						rf.mu.Lock()
						rf.stat = STATE_LEADER
						//初始化发送消息的两个数组,准备发送消息
						rf.nextIndex = make([]int,len(rf.peers))
						rf.matchIndex = make([]int,len(rf.peers))
						for i := range rf.peers {
							rf.nextIndex[i] = rf.getLastIndex() + 1
							rf.matchIndex[i] = 0
						}
						rf.mu.Unlock()
					case <- rf.heartbeatChan:
					//收到其他人的权威声明
						rf.mu.Lock()
						rf.stat = STATE_FOLLOWER
						rf.mu.Unlock()
					case <-time.After(time.Duration(rand.Int63() % 333 + 550) * time.Millisecond):
					//分票超时
						rf.mu.Lock()
						rf.stat = STATE_FOLLOWER
						rf.mu.Unlock()
					}
				}
			}
		}
		return
	}()

	// 还需要一个线程更新自己的commitedIndex和其他东西。并且reply applych

	return rf
}

func (rf *Raft) getLastIndex() int {

	return rf.log[(len(rf.log)-1)].LogIndex
}


func (rf *Raft) getLastTerm() int {

	return rf.log[(len(rf.log)-1)].LogTerm
}