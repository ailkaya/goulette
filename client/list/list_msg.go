package list

// import "fmt"

// type ListForMsg struct {
// 	head *NodeOfMsg
// 	tail *NodeOfMsg
// 	flag chan bool
// }

// type NodeOfMsg struct {
// 	Mid  int64
// 	Msg  []byte
// 	next *NodeOfMsg
// }

// func NewListForMsg() *ListForMsg {
// 	return &ListForMsg{
// 		head: nil,
// 		tail: nil,
// 		flag: make(chan bool, MaxSize),
// 	}
// }

// func (l *ListForMsg) Push(mid int64, msg []byte) {
// 	if msg == nil {
// 		return
// 	}
// 	newlist := GetNodeOfMsg()
// 	newlist.Mid, newlist.Msg, newlist.next = mid, msg, nil
// 	if l.head == nil {
// 		l.head = newlist
// 		l.tail = newlist
// 	} else {
// 		l.tail.next = newlist
// 		l.tail = newlist
// 	}
// 	l.flag <- true
// }

// func (l *ListForMsg) Pop() *NodeOfMsg {
// 	<-l.flag
// 	if l.head == nil {
// 		panic("list is empty")
// 	}
// 	node := l.head
// 	l.head = l.head.next
// 	if l.head == nil {
// 		l.tail = nil
// 	}
// 	return node
// }

// func (l *ListForMsg) GetHeadValue() int64 {
// 	// 阻塞至链表内有数据可读
// 	l.flag <- <-l.flag
// 	if l.head == nil {
// 		fmt.Println("list len:", len(l.flag))
// 		panic("list is empty")
// 	}
// 	return l.head.Mid
// }

import (
	"sync/atomic"
	"unsafe"
)

type NodeOfMsg struct {
	Mid  int64
	Msg  []byte
	next *NodeOfMsg
}

type ListForMsg struct {
	head *NodeOfMsg
	tail *NodeOfMsg
	// 标识当前链表内的数据
	flag chan bool
}

func NewListForMsg() *ListForMsg {
	dummy := &NodeOfMsg{}
	return &ListForMsg{
		head: dummy,
		tail: dummy,
		flag: make(chan bool, MaxSize),
	}
}

func (l *ListForMsg) Empty() bool {
	return l.head == nil
}

func (l *ListForMsg) Push(mid int64, msg []byte) {
	newNode := GetNodeOfMsg()
	newNode.Mid, newNode.Msg, newNode.next = mid, msg, nil
	for {
		tail := l.tail
		next := tail.next
		if tail == l.tail { // 防止跨线程修改
			if next == nil {
				// 尝试将新节点添加到链表末尾
				if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&tail.next)), nil, unsafe.Pointer(newNode)) {
					// 添加成功，尝试将尾指针移动到新节点
					atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&l.tail)), unsafe.Pointer(tail), unsafe.Pointer(newNode))
					break
				}
			} else {
				// 尾指针已被更新，推进到下一个节点
				atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&l.tail)), unsafe.Pointer(tail), unsafe.Pointer(next))
			}
		}
	}
	l.flag <- true
}

func (l *ListForMsg) Pop() *NodeOfMsg {
	// 阻塞至链表内有数据
	<-l.flag
	for {
		if l.head == nil {
			panic("list head is empty")
		}
		head := l.head
		next := head.next
		if head == l.head { // 检查头节点是否未被改变
			// 尝试将头指针移动到下一个节点
			if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&l.head)), unsafe.Pointer(head), unsafe.Pointer(next)) {
				return head
			}
		}
	}
}

// GetHeadValue 获取头节点的值
func (l *ListForMsg) GetHeadValue() int64 {
	// 阻塞至链表的head节点不为空
	l.flag <- <-l.flag
	for {
		if l.head == nil {
			panic("list head is empty")
		}
		head := l.head
		if head == l.head { // 检查头节点是否未被改变
			return head.Mid
		}
	}
}
