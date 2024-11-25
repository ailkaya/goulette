package list

// type ListForMid struct {
// 	sync.Mutex
// 	head *NodeOfMid
// 	tail *NodeOfMid
// 	flag chan bool
// }

// type NodeOfMid struct {
// 	Mid  int64
// 	next *NodeOfMid
// }

// func NewListForMid() *ListForMid {
// 	return &ListForMid{
// 		head: nil,
// 		tail: nil,
// 		flag: make(chan bool, MaxSize),
// 	}
// }

// func (l *ListForMid) Push(mid int64) {
// 	newlist := GetNodeOfMid()
// 	newlist.Mid, newlist.next = mid, nil
// 	if l.head == nil {
// 		l.head = newlist
// 		l.tail = newlist
// 	} else {
// 		l.tail.next = newlist
// 		l.tail = newlist
// 	}
// 	l.flag <- true
// }

// func (l *ListForMid) Pop() *NodeOfMid {
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

// // 获取头结点的值
// func (l *ListForMid) GetHeadValue() int64 {
// 	// 阻塞至链表内有数据可读
// 	l.flag <- <-l.flag
// 	if l.head == nil {
// 		// fmt.Println("mid cnt:", atomic.LoadInt64(&cnt))
// 		panic("list is empty")
// 	}
// 	return l.head.Mid
// }

import (
	"sync/atomic"
	"unsafe"
)

// NodeOfMid 是链表节点结构
type NodeOfMid struct {
	Mid  int64
	next *NodeOfMid
}

// ListForMid 是无锁链表结构
type ListForMid struct {
	head *NodeOfMid // 头指针
	tail *NodeOfMid // 尾指针
	// 标识当前链表内的数据
	flag chan bool
	// 标识当前链表的pop函数是否被占用
	// occupyPop int64
}

// NewListForMid 创建一个新的无锁链表
func NewListForMid() *ListForMid {
	dummy := &NodeOfMid{}
	return &ListForMid{
		head: dummy,
		tail: dummy,
		flag: make(chan bool, MaxSize),
		// occupyPop: 0,
	}
}

func (l *ListForMid) Empty() bool {
	return l.head == nil
}

// Push 将新的值插入到链表中
func (l *ListForMid) Push(mid int64) {
	newNode := GetNodeOfMid()
	newNode.Mid, newNode.next = mid, nil
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

// Pop 弹出链表头节点
func (l *ListForMid) Pop() *NodeOfMid {
	// 防止并发问题
	// 试图占据pop函数的使用权
	// for !atomic.CompareAndSwapInt64(&l.occupyPop, 0, 1) {
	// }
	// defer atomic.CompareAndSwapInt64(&l.occupyPop, 1, 0)
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
func (l *ListForMid) GetHeadValue() int64 {
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
