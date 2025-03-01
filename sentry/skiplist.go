package sentry

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Ring interface {
	AddTopicNode(location uint64, alive *int64)
	AddBrokerNode(addr string, location uint64, status *int64)
	RemoveNode(location uint64)
	GetNextBroker(location uint64, forConsumer bool) (string, error)
	Clean()
}

// Ring 的环实现，使用跳表实现
type skipList struct {
	dummy *floatPile
	// 跳表的最大层数(>=1)
	maxHeight int
	// 每一层与上一层的比例
	ratio int
	// 随机数生成器
	r *rand.Rand
}

func newSkipList(maxHeight, ratio int) (sl *skipList) {
	sl = &skipList{
		dummy: &floatPile{
			alive:    new(int64),
			location: 0,
			layers:   make([]*floatPile, maxHeight+1),
		},
		maxHeight: maxHeight,
		ratio:     ratio,
		r:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	*sl.dummy.alive = -1
	sl.dummy.lastNode, sl.dummy.nextNode = sl.dummy, sl.dummy
	for fp := range sl.dummy.layers {
		sl.dummy.layers[fp] = sl.dummy
	}
	return
}

func (sl *skipList) AddTopicNode(location uint64, alive *int64) {
	// 定位到最接近location的floatPile(<location)
	fp := sl.locateFloatPile(location)
	// 注意此处需要向前走一步，否则fp==dummy时会在错误的位置插入
	p := fp.getNextNode()
	// 定位到最接近location的node(>=location)
	for p.locate() < location && p != sl.dummy {
		p = p.getNextNode()
	}
	sl.headInsert(p, &pile{
		location:      location,
		alive:         alive,
		nextFloatPile: fp.layers[1],
	})
}

func (sl *skipList) AddBrokerNode(addr string, location uint64, alive *int64) {
	height := sl.generateHeight()
	fpLeft, path := sl.locateFloatPileWithPath(height, location)
	fpRight := fpLeft.layers[1]
	fp := &floatPile{
		alive:    alive,
		location: fpLeft.location + (fpRight.location-fpLeft.location)/2,
		source:   addr,
		layers:   make([]*floatPile, height+1),
	}
	// 加锁防止locate时访问layers导致并发问题
	fp.Lock()
	defer fp.Unlock()
	sl.headInsert(fpRight, fp)
	fp.layers[0] = fpLeft
	fpRight.layers[0] = fp

	for i := 1; i <= height; i++ {
		fp.layers[i] = path[i].layers[i]
		path[i].layers[i] = fp
	}
}

func (sl *skipList) RemoveNode(location uint64) {
	fp := sl.locateFloatPile(location)
	var p node = fp
	// 定位到最接近location的node(>=location)
	for p.locate() < location {
		p = p.getNextNode()
	}
	sl.removeNode(p)
}

func (sl *skipList) GetNextBroker(location uint64, forConsumer bool) (addr string, err error) {
	fp := sl.locateFloatPile(location).layers[1]
	nfp := fp
	if forConsumer {
		for *nfp.status() != Alive && nfp != fp {
			nfp = nfp.layers[1]
		}
	} else {
		for *nfp.status() != Alive && *nfp.status() != Closing && nfp != fp {
			nfp = nfp.layers[1]
		}
	}

	// 可能无可用的broker
	if nfp == fp {
		err = fmt.Errorf("no alive broker found")
	} else {
		addr = nfp.source
	}
	return
}

// Clean 将所有不处于正常允许状态的node移除，alive置为-1
func (sl *skipList) Clean() {
	cur, end := sl.dummy.getNextNode(), node(sl.dummy)
	for cur != end {
		alive := cur.status()
		if *alive != 1 {
			*alive = 0
			sl.removeNode(cur)
		}
		cur = cur.getNextNode()
	}
}

func (sl *skipList) locateFloatPile(location uint64) (fp *floatPile) {
	fp = sl.dummy
	curLayer := sl.maxHeight
	for curLayer > 0 {
		for fp.layers[curLayer].location < location && fp.layers[curLayer] != sl.dummy {
			fp = fp.layers[curLayer]
		}
		curLayer--
	}
	return
}

func (sl *skipList) locateFloatPileWithPath(height int, location uint64) (fp *floatPile, path []*floatPile) {
	fp = sl.dummy
	// path记录了搜索过程中从第一层到第maxHeight层所经过的所有floatPile
	path = make([]*floatPile, height+1)
	curLayer := sl.maxHeight
	for curLayer > 0 {
		for fp.layers[curLayer].location < location && fp.layers[curLayer] != sl.dummy {
			fp = fp.layers[curLayer]
		}
		if curLayer <= height {
			path[curLayer] = fp
		}
		curLayer--
	}
	return
}

// 由于写操作串行且读操作只涉及到对layers的访问，而lastNode和nextNode不受影响，因此不需要加锁(暂时)
func (sl *skipList) headInsert(pos node, newNode node) {
	newNode.setNextNode(pos)
	newNode.setLastNode(pos.getLastNode())
	pos.getLastNode().setNextNode(newNode)
	pos.setLastNode(newNode)
}

func (sl *skipList) removeNode(pos node) {
	pos.getLastNode().setNextNode(pos.getNextNode())
	pos.getNextNode().setLastNode(pos.getLastNode())
	pos.setNextNode(nil)
	pos.setLastNode(nil)
}

func (sl *skipList) generateHeight() (height int) {
	height = 1
	v := sl.r.Intn(sl.ratio)
	for v%sl.ratio == 0 && height < sl.maxHeight {
		height++
		v = sl.r.Intn(sl.ratio)
	}
	return
}

type node interface {
	// 节点类型
	category() int
	// alive状态
	status() *int64
	// 当前位置
	locate() uint64
	// 返回true代表此节点是Pile类型，应继续更新，否则结束
	setFloatPile(*floatPile) bool
	getLastNode() node
	getNextNode() node
	setLastNode(node)
	setNextNode(node)
	Lock()
	Unlock()
}

type floatPile struct {
	sync.Mutex
	// 该Broker是否存活
	alive *int64
	// 在环上所处的位置
	location uint64
	// 所属broker
	source string
	// 上/下一个Node
	lastNode node
	nextNode node
	// 存储跳表分层的相应信息，layers[0]指向上一个FloatPile(第1层)，layers[i]指向第i层(i>0)中该FloatPile的下一个FloatPile
	layers []*floatPile
}

func (fp *floatPile) category() int  { return 1 }
func (fp *floatPile) status() *int64 { return fp.alive }
func (fp *floatPile) locate() uint64 { return fp.location }
func (fp *floatPile) setFloatPile(*floatPile) bool {
	if *fp.alive == 1 {
		return false
	}
	return true
}
func (fp *floatPile) getLastNode() node     { return fp.lastNode }
func (fp *floatPile) getNextNode() node     { return fp.nextNode }
func (fp *floatPile) setLastNode(node node) { fp.lastNode = node }
func (fp *floatPile) setNextNode(node node) { fp.nextNode = node }

type pile struct {
	sync.Mutex
	location uint64
	alive    *int64
	// 当前位置的下一个floatPile
	nextFloatPile *floatPile
	// 上/下一个node(floatPile/pile)
	lastNode node
	nextNode node
}

func (p *pile) category() int  { return 2 }
func (p *pile) status() *int64 { return p.alive }
func (p *pile) locate() uint64 { return p.location }
func (p *pile) setFloatPile(nex *floatPile) bool {
	p.nextFloatPile = nex
	return true
}
func (p *pile) getLastNode() node     { return p.lastNode }
func (p *pile) getNextNode() node     { return p.nextNode }
func (p *pile) setLastNode(node node) { p.lastNode = node }
func (p *pile) setNextNode(node node) { p.nextNode = node }
