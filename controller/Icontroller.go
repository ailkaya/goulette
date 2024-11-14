package controller

type IController interface {
	RegisterTopic(string)
	SendMessage(string, []byte) error
	RetrieveMessage(string) ([]byte, error)
}
