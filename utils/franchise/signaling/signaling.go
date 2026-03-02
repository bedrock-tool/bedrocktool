package signaling

import "github.com/bedrock-tool/bedrocktool/utils/franchise/discovery"

type SignalingService struct {
	Config discovery.SignalingService
}

func NewSignalingService(discovery *discovery.Discovery) (*SignalingService, error) {
	s := &SignalingService{}
	err := discovery.Environment(&s.Config, "signaling")
	if err != nil {
		return nil, err
	}
	return s, nil
}
