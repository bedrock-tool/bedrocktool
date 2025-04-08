package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const minecraftUserAgent = "libhttpclient/1.0.0.0"

var discoveryUrls = map[string]string{
	"prod":  "https://client.discovery.minecraft-services.net/api/v1.0/discovery/MinecraftPE/builds/%s",
	"stage": "https://client.stage-b4c666a2.discovery.minecraft-services.net/api/v1.0/discovery/MinecraftPE/builds/%s",
	"dev":   "https://client.dev-11c196f7.discovery.minecraft-services.net/api/v1.0/discovery/MinecraftPE/builds/%s",
}

type Service struct {
	ServiceURI string `json:"serviceUri"`
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

type Discovery struct {
	ServiceEnvironments struct {
		Persona        map[string]ServiceWithTitle   `json:"persona"`
		Store          map[string]ServiceWithEdu     `json:"store"`
		Auth           map[string]AuthService        `json:"auth"`
		Signaling      map[string]SignalingService   `json:"signaling"`
		Filetocloud    map[string]Service            `json:"filetocloud"`
		Safety         map[string]Service            `json:"safety"`
		Mpsas          map[string]Service            `json:"mpsas"`
		Gatherings     map[string]*GatheringsService `json:"gatherings"`
		Messaging      map[string]Service            `json:"messaging"`
		Entitlements   map[string]ServiceWithTitle   `json:"entitlements"`
		Frontend       map[string]Service            `json:"frontend"`
		Multiplayer    map[string]Service            `json:"multiplayer"`
		Cdn            map[string]Service            `json:"cdn"`
		Realmsfrontend map[string]ServiceWithTitle   `json:"realmsfrontend"`
	} `json:"serviceEnvironments"`
	SupportedEnvironments map[string][]string `json:"supportedEnvironments"`

	env string `json:"-"`
}

func (d *Discovery) PersonaService() (ServiceWithTitle, error) {
	if service, ok := d.ServiceEnvironments.Persona[d.env]; ok {
		return service, nil
	}
	return ServiceWithTitle{}, errors.New("persona service not found for the environment")
}

func (d *Discovery) StoreService() (ServiceWithEdu, error) {
	if service, ok := d.ServiceEnvironments.Store[d.env]; ok {
		return service, nil
	}
	return ServiceWithEdu{}, errors.New("store service not found for the environment")
}

func (d *Discovery) AuthService() (AuthService, error) {
	if service, ok := d.ServiceEnvironments.Auth[d.env]; ok {
		return service, nil
	}
	return AuthService{}, errors.New("auth service not found for the environment")
}

func (d *Discovery) SignalingService() (SignalingService, error) {
	if service, ok := d.ServiceEnvironments.Signaling[d.env]; ok {
		return service, nil
	}
	return SignalingService{}, errors.New("signaling service not found for the environment")
}

func (d *Discovery) FiletocloudService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Filetocloud[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("filetocloud service not found for the environment")
}

func (d *Discovery) SafetyService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Safety[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("safety service not found for the environment")
}

func (d *Discovery) MpsasService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Mpsas[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("mpsas service not found for the environment")
}

func (d *Discovery) GatheringsService() (*GatheringsService, error) {
	if service, ok := d.ServiceEnvironments.Gatherings[d.env]; ok {
		return service, nil
	}
	return nil, errors.New("gatherings service not found for the environment")
}

func (d *Discovery) MessagingService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Messaging[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("messaging service not found for the environment")
}

func (d *Discovery) EntitlementsService() (ServiceWithTitle, error) {
	if service, ok := d.ServiceEnvironments.Entitlements[d.env]; ok {
		return service, nil
	}
	return ServiceWithTitle{}, errors.New("entitlements service not found for the environment")
}

func (d *Discovery) FrontendService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Frontend[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("frontend service not found for the environment")
}

func (d *Discovery) MultiplayerService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Multiplayer[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("multiplayer service not found for the environment")
}

func (d *Discovery) CdnService() (Service, error) {
	if service, ok := d.ServiceEnvironments.Cdn[d.env]; ok {
		return service, nil
	}
	return Service{}, errors.New("CDN service not found for the environment")
}

func (d *Discovery) RealmsfrontendService() (ServiceWithTitle, error) {
	if service, ok := d.ServiceEnvironments.Realmsfrontend[d.env]; ok {
		return service, nil
	}
	return ServiceWithTitle{}, errors.New("realmsfrontend service not found for the environment")
}

func GetDiscovery(env string) (*Discovery, error) {
	discoveryUrl, ok := discoveryUrls[env]
	if !ok {
		return nil, fmt.Errorf("%s is not a valid env", env)
	}

	resp, err := http.DefaultClient.Get(fmt.Sprintf(discoveryUrl, protocol.CurrentVersion))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Result Discovery
	}
	d := json.NewDecoder(resp.Body)
	err = d.Decode(&data)
	if err != nil {
		return nil, err
	}

	data.Result.env = data.Result.SupportedEnvironments[protocol.CurrentVersion][0]

	return &data.Result, nil
}
