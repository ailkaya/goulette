package handle

import (
	"context"

	"github.com/ailkaya/goport/client"
)

type PreProcessor struct {
	controller client.IPreProcessorController
	ctx        context.Context
	maxReceive int
	hasReceive int
	in         chan []byte
	block      chan bool
}

func NewPreProcessor(ctx context.Context, maxReceive int) *PreProcessor {
	return &PreProcessor{
		ctx:        ctx,
		maxReceive: maxReceive,
		hasReceive: maxReceive,
	}
}

func (p *PreProcessor) Cite(r client.IPreProcessorController) client.IPreProcessor {
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
		p.controller.Runnable()
		<-p.block
		p.hasReceive = -1
	}
	p.hasReceive++
	select {
	case data := <-p.in:
		return data
	case <-p.ctx.Done():
		// fmt.Println("PreProcessor Done")
		return nil
	}
	// return <-p.in
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
