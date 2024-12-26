package client

type PreProcessor struct {
	controller IPreProcessorController
	maxReceive int
	hasReceive int
	in         chan []byte
	block      chan bool
}

func NewPreProcessor(maxReceive int) *PreProcessor {
	return &PreProcessor{
		maxReceive: maxReceive,
		hasReceive: maxReceive,
	}
}

func (p *PreProcessor) Cite(r IPreProcessorController) IPreProcessor {
	p.controller = r
	return p
}

func (p *PreProcessor) Register(in chan []byte) chan bool {
	p.in = in
	p.block = make(chan bool, 1)
	return p.block
}

func (p *PreProcessor) Process() []byte {
	if p.hasReceive+1 >= p.maxReceive {
		// fmt.Println(2)
		p.controller.Runnable()
		// fmt.Println(3)
		<-p.block
		// fmt.Println(1)
		// fmt.Println(time.Now())
		p.hasReceive = -1
	}
	p.hasReceive++
	return <-p.in
	// fmt.Println(<-p.in)
	// return []byte{}
}

func (p *PreProcessor) Close() {
	close(p.block)
}

type PostProcessor struct {
	out chan []byte
}

func NewPostProcessor() *PostProcessor {
	return &PostProcessor{}
}

func (p *PostProcessor) Register(out chan []byte) {
	p.out = out
}

func (p *PostProcessor) Process(data []byte) {
	p.out <- data
}

func (p *PostProcessor) Close() {

}
