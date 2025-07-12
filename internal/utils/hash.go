package utils

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
)

// NodeIterator 哈希环节点迭代器
type NodeIterator struct {
	hashRing *HashRing
	key      string
	startIdx int
	currIdx  int
	seen     map[string]bool
	started  bool // 标记是否已经开始迭代
}

// HashRing 一致性哈希环
type HashRing struct {
	nodes    []uint32
	nodeMap  map[uint32]string
	replicas int
}

// NewHashRing 创建新的哈希环
func NewHashRing(replicas int) *HashRing {
	return &HashRing{
		nodes:    make([]uint32, 0),
		nodeMap:  make(map[uint32]string),
		replicas: replicas,
	}
}

// AddNode 添加节点到哈希环
func (h *HashRing) AddNode(node string) {
	for i := 0; i < h.replicas; i++ {
		virtualNode := node + ":" + strconv.Itoa(i)
		hash := h.hash(virtualNode)
		h.nodes = append(h.nodes, hash)
		h.nodeMap[hash] = node
	}
	sort.Slice(h.nodes, func(i, j int) bool {
		return h.nodes[i] < h.nodes[j]
	})
}

// RemoveNode 从哈希环移除节点
func (h *HashRing) RemoveNode(node string) {
	for i := 0; i < h.replicas; i++ {
		virtualNode := node + ":" + strconv.Itoa(i)
		hash := h.hash(virtualNode)

		// 从nodes切片中移除
		for j, nodeHash := range h.nodes {
			if nodeHash == hash {
				h.nodes = append(h.nodes[:j], h.nodes[j+1:]...)
				break
			}
		}

		// 从nodeMap中移除
		delete(h.nodeMap, hash)
	}
}

// GetNodeIter 获取节点迭代器
func (h *HashRing) GetNodeIter(key string) *NodeIterator {
	if len(h.nodes) == 0 {
		return &NodeIterator{
			hashRing: h,
			key:      key,
			startIdx: 0,
			currIdx:  0,
			seen:     make(map[string]bool),
			started:  false,
		}
	}

	hash := h.hash(key)
	idx := sort.Search(len(h.nodes), func(i int) bool {
		return h.nodes[i] >= hash
	})

	if idx == len(h.nodes) {
		idx = 0
	}

	return &NodeIterator{
		hashRing: h,
		key:      key,
		startIdx: idx,
		currIdx:  idx,
		seen:     make(map[string]bool),
		started:  false,
	}
}

// Next 获取下一个节点
func (iter *NodeIterator) Next() (string, error) {
	if len(iter.hashRing.nodes) == 0 {
		return "", nil
	}

	// 遍历哈希环一圈
	for i := 0; i < len(iter.hashRing.nodes); i++ {
		nodeIdx := (iter.currIdx + i) % len(iter.hashRing.nodes)
		node := iter.hashRing.nodeMap[iter.hashRing.nodes[nodeIdx]]

		if !iter.seen[node] {
			iter.seen[node] = true
			iter.currIdx = (nodeIdx + 1) % len(iter.hashRing.nodes)

			// 检查是否再次到达开始位置
			if iter.started && iter.currIdx == iter.startIdx {
				return node, fmt.Errorf("已遍历完所有节点，再次到达开始位置")
			}

			iter.started = true
			return node, nil
		}
	}

	return "", nil
}

// Reset 重置迭代器
func (iter *NodeIterator) Reset() {
	iter.currIdx = iter.startIdx
	iter.seen = make(map[string]bool)
	iter.started = false
}

// GetNode 根据key获取对应的节点（保持向后兼容）
func (h *HashRing) GetNode(key string) string {
	iter := h.GetNodeIter(key)
	node, _ := iter.Next()
	return node
}

// GetNodes 获取指定数量的节点（保持向后兼容）
func (h *HashRing) GetNodes(key string, count int) []string {
	if len(h.nodes) == 0 {
		return []string{}
	}

	iter := h.GetNodeIter(key)
	nodes := make([]string, 0, count)

	for i := 0; i < count; i++ {
		node, err := iter.Next()
		if node == "" || err != nil {
			break
		}
		nodes = append(nodes, node)
	}

	return nodes
}

// hash 计算字符串的哈希值
func (h *HashRing) hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

// MD5Hash 计算MD5哈希
func MD5Hash(data []byte) []byte {
	hash := md5.Sum(data)
	return hash[:]
}
