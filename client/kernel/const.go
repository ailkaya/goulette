package kernel

const (
	// send但不确认, 不保证到达
	// sendWithoutAck                 = "sendWithoutAck"
	// send确认, 保证到达
	SendWithGuaranteedArrival = "sendWithGuaranteedArrival"
	// receive不确认, 不保证到达
	// receiveWithoutAck              = "receiveWithoutAck"
	// receive确认, 保证接收
	ReceiveWithGuaranteedReception = "receiveWithGuaranteedReception"

	ErrorStreamClosed = "rpc error: code = Internal desc = SendMsg called after CloseSend"
)
