package codefresh

type codefreshClient struct {
}

type CodefreshClient interface {
	Send(payload []byte) error
}

func NewCodefreshClient() CodefreshClient {
	return &codefreshClient{}
}

func (cc *codefreshClient) Send(payload []byte) error {
	return nil
}
