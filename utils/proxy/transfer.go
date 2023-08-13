package proxy

type errTransfer struct{}

func (errTransfer) Error() string {
	return "errTransfer"
}
