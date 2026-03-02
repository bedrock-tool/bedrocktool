package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bedrock-tool/bedrocktool/utils/franchise/internal"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

var discoveryUrls = map[string]string{
	"prod":  "https://client.discovery.minecraft-services.net/api/v1.0/discovery/MinecraftPE/builds/%s",
	"stage": "https://client.stage-b4c666a2.discovery.minecraft-services.net/api/v1.0/discovery/MinecraftPE/builds/%s",
	"dev":   "https://client.dev-11c196f7.discovery.minecraft-services.net/api/v1.0/discovery/MinecraftPE/builds/%s",
}

type Service struct {
	ServiceURI string `json:"serviceUri"`
}

func (s Service) Url(format string, a ...any) string {
	return s.ServiceURI + fmt.Sprintf(format, a...)
}

type ServiceWithTitle struct {
	Service
	PlayfabTitleID string `json:"playfabTitleId"`
}

type ServiceWithEdu struct {
	ServiceWithTitle
	EduPlayFabTitleID string `json:"eduPlayFabTitleId"`
}

type SignalingService struct {
	Service
	StunURI string `json:"stunUri"`
	TurnURI string `json:"turnUri"`
}

type AuthService struct {
	ServiceWithEdu
	Issuer string `json:"issuer"`
}

type Discovery struct {
	ServiceEnvironments   map[string]map[string]json.RawMessage `json:"serviceEnvironments"`
	SupportedEnvironments map[string][]string                   `json:"supportedEnvironments"`

	Env string `json:"-"`
}

func (d *Discovery) Environment(env any, name string) error {
	e, ok := d.ServiceEnvironments[name]
	if !ok {
		return errors.New("minecraft/franchise: environment not found")
	}
	data, ok := e[d.Env]
	if !ok {
		return errors.New("minecraft/franchise: environment with type not found")
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode environment: %w", err)
	}
	return nil
}

const (
	EnvironmentTypeProduction  = "prod"
	EnvironmentTypeDevelopment = "dev"
	EnvironmentTypeStaging     = "stage"
)

func GetDiscovery(ctx context.Context, env string) (*Discovery, error) {
	discoveryUrl, ok := discoveryUrls[env]
	if !ok {
		return nil, fmt.Errorf("%s is not a valid env", env)
	}

	resp, err := internal.DoRequest[internal.Result[Discovery]](
		ctx, http.DefaultClient, "GET",
		fmt.Sprintf(discoveryUrl, protocol.CurrentVersion), nil, nil)
	if err != nil {
		return nil, err
	}

	resp.Data.Env = resp.Data.SupportedEnvironments[protocol.CurrentVersion][0]
	return &resp.Data, nil
}
