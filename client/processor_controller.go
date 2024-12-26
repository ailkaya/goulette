package client

import (
	"sync"
	// "time"
)

// 要使用时需要首先调用Start()方法
type PreProcessorController struct {
	sync.Mutex
	// in chan []byte
	// 已注册的容器数量
	registered int
	// 正在运行的容器数量(已经开始接收数据的container数量)
	running int
	// 本周期已经开始等待的容器数量
	waiting int
	// 用于block的通道
	blocks []chan bool
}

func NewPreProcessorController() *PreProcessorController {
	return &PreProcessorController{
		registered: 0,
		running:    0,
		waiting:    0,
		blocks:     make([]chan bool, 0),
	}
}

func (p *PreProcessorController) IPreProcessorControllerImpl() {}

func (p *PreProcessorController) Start() {
	p.Lock()
	defer p.Unlock()
	// fmt.Println("start")
	p.nextCycle()
}

func (p *PreProcessorController) nextCycle() {
	// fmt.Println("next cycle", p.registered, p.running, len(p.blocks))
	for i, block := range p.blocks {
		// 判断container是否已经关闭
		// fmt.Println(block)
		select {
		case _, ok := <-block:
			if !ok {
				// fmt.Println(6)
				// 从blocks中移除
				p.blocks = append(p.blocks[:i], p.blocks[i+1:]...)
				continue
			}
		default:
		}
		block <- true
	}
	// fmt.Println()
	// time.Sleep(time.Millisecond * 200)
	// 更新正在运行的container数量
	p.running = p.registered
}

func (p *PreProcessorController) Runnable() {
	// fmt.Println("runnable")
	p.Lock()
	defer p.Unlock()
	p.waiting++
	// fmt.Println("runnable", p.waiting, p.running)

	if p.waiting >= p.running {
		p.nextCycle()
		p.waiting = 0
	}
}

func (p *PreProcessorController) RegisterContainer(block chan bool) {
	p.Lock()
	defer p.Unlock()
	p.registered++
	p.blocks = append(p.blocks, block)
}

func (p *PreProcessorController) UnRegisterContainer() {
	p.registered--
}
