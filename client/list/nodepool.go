package list

import (
	"sync"
)

var (
	MaxSize       = 100000
	NodeOfMidPool = sync.Pool{
		New: func() interface{} {
			return &NodeOfMid{}
		},
	}
	NodeOfMsgPool = sync.Pool{
		New: func() interface{} {
			return &NodeOfMsg{}
		},
	}
)

func GetNodeOfMid() *NodeOfMid {
	return NodeOfMidPool.Get().(*NodeOfMid)
}

func PutNodeOfMid(node *NodeOfMid) {
	if node == nil {
		return
	}
	// node.next = nil
	NodeOfMidPool.Put(node)
}

func GetNodeOfMsg() *NodeOfMsg {
	return NodeOfMsgPool.Get().(*NodeOfMsg)
}

func PutNodeOfMsg(node *NodeOfMsg) {
	if node == nil {
		return
	}
	// node.next = nil
	NodeOfMsgPool.Put(node)
}
