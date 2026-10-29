package connectinfo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/p2p"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sandertv/gophertunnel/minecraft/service/gatherings"
)

type ErrNotFound struct {
	Name string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.Name)
}

type Kind string

const (
	KindServer     Kind = "server"
	KindRealm      Kind = "realm"
	KindExperience Kind = "experience"
	KindReplay     Kind = "replay"
	KindXbox       Kind = "xbox"
)

func (k Kind) String() string {
	return string(k)
}

type ParsedConnectInfo struct {
	Kind Kind

	RealmName  string
	Experience string
	Replay     string
	Address    string
	Xbox       string
}

type ConnectInfo struct {
	Value   string
	Account *auth.Account

	realm      *realms.Realm
	experience *gatherings.Experience
}

func (c *ConnectInfo) Parsed() (*ParsedConnectInfo, error) {
	return parseConnectInfo(c.Value)
}

func (c *ConnectInfo) Kind() (Kind, error) {
	parsed, err := c.Parsed()
	if err != nil {
		return "", err
	}
	return parsed.Kind, nil
}

func experienceName(experience *gatherings.Experience) string {
	if experience == nil {
		return ""
	}
	name := experience.Item.Title.Neutral()
	if name == "" {
		name = experience.NewsTitle
	}
	if name == "" {
		name = experience.ID.String()
	}
	return name
}

func experienceMatches(experience *gatherings.Experience, name string) bool {
	if experience == nil {
		return false
	}
	return strings.EqualFold(experienceName(experience), name) ||
		strings.EqualFold(experience.ID.String(), name)
}

func (c *ConnectInfo) getExperience(ctx context.Context, name string) (*gatherings.Experience, error) {
	name = strings.TrimSpace(name)
	if experienceMatches(c.experience, name) {
		return c.experience, nil
	}
	if c.Account == nil {
		return nil, errors.New("no account")
	}
	gatheringsClient, err := c.Account.Gatherings(ctx)
	if err != nil {
		return nil, err
	}
	experiences, err := gatheringsClient.Experiences(ctx)
	if err != nil {
		return nil, err
	}
	for _, experience := range experiences {
		if experienceMatches(experience, name) {
			c.experience = experience
			return experience, nil
		}
	}
	return nil, &ErrNotFound{Name: name}
}

func (c *ConnectInfo) getRealm(ctx context.Context, name string) (*realms.Realm, error) {
	name = strings.ToLower(name)
	if c.realm != nil && (strings.EqualFold(c.realm.Name, name) || strconv.Itoa(c.realm.ID) == name) {
		return c.realm, nil
	}
	realmsClient, err := c.Account.Realms(ctx)
	if err != nil {
		return nil, err
	}
	realms, err := realmsClient.Realms(ctx)
	if err != nil {
		return nil, err
	}
	for _, realm := range realms {
		lowerName := strings.ToLower(realm.Name)
		if strings.HasPrefix(lowerName, name) || strconv.Itoa(realm.ID) == name {
			c.realm = &realm
			return c.realm, nil
		}
	}
	return nil, &ErrNotFound{Name: name}
}

func (c *ConnectInfo) Realm(ctx context.Context) (*realms.Realm, error) {
	parsed, err := c.Parsed()
	if err != nil {
		return nil, err
	}
	if parsed.Kind != KindRealm {
		return nil, nil
	}
	return c.getRealm(ctx, parsed.RealmName)
}

func (c *ConnectInfo) Address(ctx context.Context) (string, error) {
	parsed, err := c.Parsed()
	if err != nil {
		return "", err
	}
	switch parsed.Kind {
	case KindXbox:
		return parsed.Xbox, nil
	case KindServer:
		return parsed.Address, nil
	case KindReplay:
		return parsed.Replay, nil
	case KindExperience:
		experience, err := c.getExperience(ctx, parsed.Experience)
		if err != nil {
			return "", err
		}
		address, err := experience.Join(ctx)
		if err != nil {
			return "", fmt.Errorf("JoinExperience: %w", err)
		}
		return normalizeAddress(address.String())
	case KindRealm:
		realm, err := c.getRealm(ctx, parsed.RealmName)
		if err != nil {
			return "", err
		}
		address, err := realm.Address(ctx)
		if err != nil {
			return "", err
		}
		return normalizeAddress(address.Address)
	default:
		return "", errors.New("invalid address")
	}
}

func (c *ConnectInfo) IsReplay() bool {
	parsed, err := c.Parsed()
	return err == nil && parsed.Kind == KindReplay
}

func (c *ConnectInfo) SetRealm(realm *realms.Realm) {
	c.Value = "realm:" + realm.Name
	c.realm = realm
}

func (c *ConnectInfo) SetFeaturedServer(server *gatherings.FeaturedServer) {
	c.Value = server.Address()
}

func (c *ConnectInfo) SetExperience(experience *gatherings.Experience) {
	c.Value = "experience:" + experienceName(experience)
	c.experience = experience
}

func (c *ConnectInfo) SetWorld(world *p2p.World) {
	c.Value = "xbox:" + world.HandleID().String()
}

func parseConnectInfo(value string) (*ParsedConnectInfo, error) {
	// experience by name or ID
	if experienceRegex.MatchString(value) {
		p := regexGetParams(experienceRegex, value)
		return &ParsedConnectInfo{
			Kind:       KindExperience,
			Experience: strings.TrimSpace(p["Name"]),
		}, nil
	}

	// realm
	if realmRegex.MatchString(value) {
		p := regexGetParams(realmRegex, value)
		input := strings.ToLower(p["Name"])
		return &ParsedConnectInfo{
			Kind:      KindRealm,
			RealmName: input,
		}, nil
	}

	// pcap replay
	if pcapRegex.MatchString(value) {
		p := regexGetParams(pcapRegex, value)
		input := p["Filename"]
		return &ParsedConnectInfo{
			Kind:   KindReplay,
			Replay: input,
		}, nil
	}

	// xbox live friend session
	if xboxRegex.MatchString(value) {
		p := regexGetParams(xboxRegex, value)
		id, err := uuid.Parse(p["ID"])
		if err != nil {
			return nil, fmt.Errorf("xbox: %w", err)
		}
		return &ParsedConnectInfo{
			Kind: KindXbox,
			Xbox: id.String(),
		}, nil
	}

	// normal server dns, ip or url
	address, err := normalizeAddress(value)
	if err != nil {
		return nil, err
	}
	return &ParsedConnectInfo{
		Kind:    KindServer,
		Address: address,
	}, nil
}
