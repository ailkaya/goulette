package raft

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
)

// RaftState Raft节点状态
type RaftState int

const (
	Follower RaftState = iota
	Candidate
	Leader
)

// LogEntry 日志条目
type LogEntry struct {
	Term    uint64
	Index   uint64
	Command interface{}
}

// RaftNode Raft节点
type RaftNode struct {
	mu sync.RWMutex

	// 持久化状态
	currentTerm uint64
	votedFor    string
	log         []LogEntry

	// 易失性状态
	commitIndex uint64
	lastApplied uint64

	// Leader易失性状态
	nextIndex  map[string]uint64
	matchIndex map[string]uint64

	// 节点信息
	nodeID   string
	state    RaftState
	leaderID string
	votes    map[string]bool
	peers    map[string]string // peerID -> address

	// 选举相关
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration
	lastHeartbeat    time.Time
	electionTimer    *time.Timer

	// 回调函数
	applyFunc func(interface{}) error
	sendFunc  func(string, interface{}) error

	// 停止信号
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewRaftNode 创建新的Raft节点
func NewRaftNode(nodeID string, peers map[string]string, applyFunc func(interface{}) error, sendFunc func(string, interface{}) error) *RaftNode {
	node := &RaftNode{
		nodeID:           nodeID,
		state:            Follower,
		peers:            peers,
		votes:            make(map[string]bool),
		nextIndex:        make(map[string]uint64),
		matchIndex:       make(map[string]uint64),
		electionTimeout:  time.Duration(150+rand.Intn(150)) * time.Millisecond,
		heartbeatTimeout: 50 * time.Millisecond,
		applyFunc:        applyFunc,
		sendFunc:         sendFunc,
		stopChan:         make(chan struct{}),
		log:              []LogEntry{{Term: 0, Index: 0, Command: nil}}, // 初始空日志
	}

	// 初始化nextIndex和matchIndex
	for peerID := range peers {
		if peerID != nodeID {
			node.nextIndex[peerID] = 1
			node.matchIndex[peerID] = 0
		}
	}

	return node
}

// Start 启动Raft节点
func (node *RaftNode) Start() {
	node.wg.Add(1)
	go node.run()
	utils.Infof("Raft节点启动: %s", node.nodeID)
}

// Stop 停止Raft节点
func (node *RaftNode) Stop() {
	close(node.stopChan)
	node.wg.Wait()
	utils.Infof("Raft节点停止: %s", node.nodeID)
}

// run 主运行循环
func (node *RaftNode) run() {
	defer node.wg.Done()

	for {
		select {
		case <-node.stopChan:
			return
		default:
			node.mu.Lock()
			switch node.state {
			case Follower:
				node.runFollower()
			case Candidate:
				node.runCandidate()
			case Leader:
				node.runLeader()
			}
			node.mu.Unlock()
		}
	}
}

// runFollower Follower状态运行
func (node *RaftNode) runFollower() {
	if node.electionTimer == nil {
		node.resetElectionTimer()
	}

	select {
	case <-node.electionTimer.C:
		node.startElection()
	case <-node.stopChan:
		return
	default:
		time.Sleep(10 * time.Millisecond)
	}
}

// runCandidate Candidate状态运行
func (node *RaftNode) runCandidate() {
	if node.electionTimer == nil {
		node.resetElectionTimer()
	}

	select {
	case <-node.electionTimer.C:
		node.startElection()
	case <-node.stopChan:
		return
	default:
		time.Sleep(10 * time.Millisecond)
	}
}

// runLeader Leader状态运行
func (node *RaftNode) runLeader() {
	// 发送心跳
	node.sendHeartbeat()
	time.Sleep(node.heartbeatTimeout)
}

// startElection 开始选举
func (node *RaftNode) startElection() {
	node.currentTerm++
	node.state = Candidate
	node.votedFor = node.nodeID
	node.votes = make(map[string]bool)
	node.votes[node.nodeID] = true

	utils.Infof("节点 %s 开始选举，任期: %d", node.nodeID, node.currentTerm)

	// 重置选举定时器
	node.resetElectionTimer()

	// 向其他节点请求投票
	for peerID := range node.peers {
		if peerID != node.nodeID {
			node.wg.Add(1)
			go func(peerID string) {
				defer node.wg.Done()
				node.requestVote(peerID)
			}(peerID)
		}
	}
}

// requestVote 请求投票
func (node *RaftNode) requestVote(peerID string) {
	request := &RequestVoteRequest{
		Term:         node.currentTerm,
		CandidateID:  node.nodeID,
		LastLogIndex: node.getLastLogIndex(),
		LastLogTerm:  node.getLastLogTerm(),
	}

	if err := node.sendFunc(peerID, request); err != nil {
		utils.Warnf("向节点 %s 请求投票失败: %v", peerID, err)
	}
}

// sendHeartbeat 发送心跳
func (node *RaftNode) sendHeartbeat() {
	for peerID := range node.peers {
		if peerID != node.nodeID {
			node.wg.Add(1)
			go func(peerID string) {
				defer node.wg.Done()
				node.sendAppendEntries(peerID, true) // 空日志条目作为心跳
			}(peerID)
		}
	}
}

// sendAppendEntries 发送追加日志条目
func (node *RaftNode) sendAppendEntries(peerID string, isHeartbeat bool) {
	nextIndex := node.nextIndex[peerID]
	entries := []LogEntry{}

	if !isHeartbeat {
		// 获取需要发送的日志条目
		if nextIndex <= uint64(len(node.log)) {
			entries = node.log[nextIndex-1:]
		}
	}

	request := &AppendEntriesRequest{
		Term:         node.currentTerm,
		LeaderID:     node.nodeID,
		PrevLogIndex: nextIndex - 1,
		PrevLogTerm:  node.getLogTerm(nextIndex - 1),
		Entries:      entries,
		LeaderCommit: node.commitIndex,
	}

	if err := node.sendFunc(peerID, request); err != nil {
		utils.Warnf("向节点 %s 发送追加日志失败: %v", peerID, err)
	}
}

// resetElectionTimer 重置选举定时器
func (node *RaftNode) resetElectionTimer() {
	if node.electionTimer != nil {
		node.electionTimer.Stop()
	}
	node.electionTimer = time.NewTimer(node.electionTimeout)
}

// getLastLogIndex 获取最后日志索引
func (node *RaftNode) getLastLogIndex() uint64 {
	return uint64(len(node.log) - 1)
}

// getLastLogTerm 获取最后日志任期
func (node *RaftNode) getLastLogTerm() uint64 {
	if len(node.log) == 0 {
		return 0
	}
	return node.log[len(node.log)-1].Term
}

// getLogTerm 获取指定索引的日志任期
func (node *RaftNode) getLogTerm(index uint64) uint64 {
	if index == 0 || index > uint64(len(node.log)) {
		return 0
	}
	return node.log[index-1].Term
}

// Propose 提议新命令
func (node *RaftNode) Propose(command interface{}) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	if node.state != Leader {
		return fmt.Errorf("只有leader可以提议命令")
	}

	// 添加日志条目
	entry := LogEntry{
		Term:    node.currentTerm,
		Index:   node.getLastLogIndex() + 1,
		Command: command,
	}
	node.log = append(node.log, entry)

	utils.Debugf("节点 %s 提议命令: index=%d, term=%d", node.nodeID, entry.Index, entry.Term)

	// 复制到其他节点
	for peerID := range node.peers {
		if peerID != node.nodeID {
			node.wg.Add(1)
			go func(peerID string) {
				defer node.wg.Done()
				node.sendAppendEntries(peerID, false)
			}(peerID)
		}
	}

	return nil
}

// HandleRequestVote 处理投票请求
func (node *RaftNode) HandleRequestVote(request *RequestVoteRequest) *RequestVoteResponse {
	node.mu.Lock()
	defer node.mu.Unlock()

	response := &RequestVoteResponse{
		Term:        node.currentTerm,
		VoteGranted: false,
	}

	// 如果请求任期小于当前任期，拒绝投票
	if request.Term < node.currentTerm {
		return response
	}

	// 如果请求任期大于当前任期，转换为follower
	if request.Term > node.currentTerm {
		node.currentTerm = request.Term
		node.state = Follower
		node.votedFor = ""
		node.leaderID = ""
	}

	// 检查是否可以投票
	if (node.votedFor == "" || node.votedFor == request.CandidateID) &&
		node.isLogUpToDate(request.LastLogIndex, request.LastLogTerm) {
		node.votedFor = request.CandidateID
		response.VoteGranted = true
		node.resetElectionTimer()
		utils.Infof("节点 %s 投票给 %s", node.nodeID, request.CandidateID)
	}

	return response
}

// HandleRequestVoteResponse 处理投票响应
func (node *RaftNode) HandleRequestVoteResponse(peerID string, response *RequestVoteResponse) {
	node.mu.Lock()
	defer node.mu.Unlock()

	// 如果响应任期大于当前任期，转换为follower
	if response.Term > node.currentTerm {
		node.currentTerm = response.Term
		node.state = Follower
		node.votedFor = ""
		node.leaderID = ""
		return
	}

	// 如果当前不是candidate或任期不匹配，忽略响应
	if node.state != Candidate || response.Term != node.currentTerm {
		return
	}

	if response.VoteGranted {
		node.votes[peerID] = true
		utils.Debugf("节点 %s 获得来自 %s 的投票", node.nodeID, peerID)

		// 检查是否获得多数票
		voteCount := len(node.votes)
		if voteCount > len(node.peers)/2 {
			node.becomeLeader()
		}
	}
}

// HandleAppendEntries 处理追加日志请求
func (node *RaftNode) HandleAppendEntries(request *AppendEntriesRequest) *AppendEntriesResponse {
	node.mu.Lock()
	defer node.mu.Unlock()

	response := &AppendEntriesResponse{
		Term:    node.currentTerm,
		Success: false,
	}

	// 如果请求任期小于当前任期，拒绝
	if request.Term < node.currentTerm {
		return response
	}

	// 如果请求任期大于当前任期，转换为follower
	if request.Term > node.currentTerm {
		node.currentTerm = request.Term
		node.state = Follower
		node.votedFor = ""
	}

	// 更新leader ID和重置选举定时器
	node.leaderID = request.LeaderID
	node.resetElectionTimer()

	// 检查日志一致性
	if request.PrevLogIndex > 0 {
		if request.PrevLogIndex > node.getLastLogIndex() ||
			node.getLogTerm(request.PrevLogIndex) != request.PrevLogTerm {
			return response
		}
	}

	// 追加日志条目
	for i, entry := range request.Entries {
		index := request.PrevLogIndex + uint64(i) + 1
		if index <= uint64(len(node.log)) {
			if node.log[index-1].Term != entry.Term {
				// 删除冲突的日志条目及其后续条目
				node.log = node.log[:index-1]
			}
		}
		if index > uint64(len(node.log)) {
			node.log = append(node.log, entry)
		}
	}

	// 更新commitIndex
	if request.LeaderCommit > node.commitIndex {
		node.commitIndex = min(request.LeaderCommit, node.getLastLogIndex())
		node.applyLogs()
	}

	response.Success = true
	return response
}

// HandleAppendEntriesResponse 处理追加日志响应
func (node *RaftNode) HandleAppendEntriesResponse(peerID string, response *AppendEntriesResponse) {
	node.mu.Lock()
	defer node.mu.Unlock()

	// 如果响应任期大于当前任期，转换为follower
	if response.Term > node.currentTerm {
		node.currentTerm = response.Term
		node.state = Follower
		node.votedFor = ""
		node.leaderID = ""
		return
	}

	// 如果当前不是leader或任期不匹配，忽略响应
	if node.state != Leader || response.Term != node.currentTerm {
		return
	}

	if response.Success {
		// 更新nextIndex和matchIndex
		node.nextIndex[peerID] = node.nextIndex[peerID] + 1
		node.matchIndex[peerID] = node.nextIndex[peerID] - 1

		// 尝试提交新的日志条目
		node.tryCommitLogs()
	} else {
		// 减少nextIndex重试
		if node.nextIndex[peerID] > 1 {
			node.nextIndex[peerID]--
		}
	}
}

// becomeLeader 成为leader
func (node *RaftNode) becomeLeader() {
	node.state = Leader
	node.leaderID = node.nodeID
	utils.Infof("节点 %s 成为leader，任期: %d", node.nodeID, node.currentTerm)

	// 初始化leader状态
	for peerID := range node.peers {
		if peerID != node.nodeID {
			node.nextIndex[peerID] = node.getLastLogIndex() + 1
			node.matchIndex[peerID] = 0
		}
	}

	// 发送初始心跳
	node.sendHeartbeat()
}

// applyLogs 应用日志条目
func (node *RaftNode) applyLogs() {
	for node.lastApplied < node.commitIndex {
		node.lastApplied++
		entry := node.log[node.lastApplied-1]
		if node.applyFunc != nil {
			if err := node.applyFunc(entry.Command); err != nil {
				utils.Errorf("应用日志条目失败: index=%d, error=%v", node.lastApplied, err)
			}
		}
	}
}

// tryCommitLogs 尝试提交日志条目
func (node *RaftNode) tryCommitLogs() {
	for i := node.commitIndex + 1; i <= node.getLastLogIndex(); i++ {
		if node.log[i-1].Term == node.currentTerm {
			count := 1 // 包括leader自己
			for peerID := range node.peers {
				if peerID != node.nodeID && node.matchIndex[peerID] >= i {
					count++
				}
			}
			if count > len(node.peers)/2 {
				node.commitIndex = i
			}
		}
	}
	node.applyLogs()
}

// isLogUpToDate 检查日志是否是最新的
func (node *RaftNode) isLogUpToDate(lastLogIndex, lastLogTerm uint64) bool {
	myLastLogTerm := node.getLastLogTerm()
	if lastLogTerm != myLastLogTerm {
		return lastLogTerm > myLastLogTerm
	}
	return lastLogIndex >= node.getLastLogIndex()
}

// GetState 获取当前状态
func (node *RaftNode) GetState() (term uint64, isLeader bool) {
	node.mu.RLock()
	defer node.mu.RUnlock()
	return node.currentTerm, node.state == Leader
}

// GetLeaderID 获取当前leader ID
func (node *RaftNode) GetLeaderID() string {
	node.mu.RLock()
	defer node.mu.RUnlock()
	return node.leaderID
}

// 辅助函数
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// RequestVoteRequest 投票请求
type RequestVoteRequest struct {
	Term         uint64
	CandidateID  string
	LastLogIndex uint64
	LastLogTerm  uint64
}

// RequestVoteResponse 投票响应
type RequestVoteResponse struct {
	Term        uint64
	VoteGranted bool
}

// AppendEntriesRequest 追加日志请求
type AppendEntriesRequest struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []LogEntry
	LeaderCommit uint64
}

// AppendEntriesResponse 追加日志响应
type AppendEntriesResponse struct {
	Term    uint64
	Success bool
}
