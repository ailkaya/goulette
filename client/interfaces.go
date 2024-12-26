package client

// pb "github.com/ailkaya/goport/broker"

// client参数在初始化阶段传入
type IKernel interface {
	Need(push func([]byte), pull func() []byte)
	Type() string
	InitConfig() error
	Run()
	Close()
}

type IContainerConfig interface {
	GetConfig() IContainerConfig
}

// fa: IPreProcessor, IPostProcessor
type IContainer interface {
	Cite(IKernel, IPreProcessor, IPostProcessor)
	// GetClient() pb.BrokerServiceClient
	Run() error
	Close()
	// PreProcess() []byte
	// PostProcess([]byte)
}

// cite: IReception
// 前置处理器接口
// 主要功能：负载均衡, 数据统计等
type IPreProcessor interface {
	Cite(IPreProcessorController) IPreProcessor
	// 传入一个周期内的最大接收数和数据输入通道, 返回实现阻塞所需要的通道
	Register(chan []byte) chan bool
	Process() []byte
	Close()
}

// cite: IReception
// 后置处理器接口
type IPostProcessor interface {
	// Cite(IReception)
	Register(chan []byte)
	Process([]byte)
	Close()
}

// type IReception interface {
// 	NewClient() *Client
// 	GetIn() chan []byte
// 	GetOut() chan []byte
// }

// 用于控制container接收对齐的接口
type IPreProcessorController interface {
	IPreProcessorControllerImpl()
	// 开始第一个周期
	Start()
	// 开启一个新周期
	nextCycle()
	// 由子类调用表示某个container已经准备好接收下一轮数据
	Runnable()
	// 新增一个container, 传入一个用于阻塞的通道
	RegisterContainer(chan bool)
	// 减少一个container
	UnRegisterContainer()
}
