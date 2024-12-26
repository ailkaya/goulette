package service

type IReception interface {
	Run() error
	Ack(int32)
	Close()
}
