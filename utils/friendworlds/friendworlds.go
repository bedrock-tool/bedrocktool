// Package friendworlds finds the worlds the signed-in account's friends are
// playing right now, the ones bedrocktool can join and download through
// their Xbox Live multiplayer session ("xbox:<handle id>" addresses).
package friendworlds

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/df-mc/go-xsapi/v2/mpsd"
	"github.com/google/uuid"
	mcauth "github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/p2p"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

// Status is whether a friend's world can be joined, and if not, why.
type Status int

const (
	Joinable Status = iota
	// Full has as many players as the host allows.
	Full
	// InviteOnly is only open to players the host invites from the game.
	InviteOnly
	// Closed isn't taking new players.
	Closed
	// OtherVersion runs a Minecraft version bedrocktool doesn't speak.
	OtherVersion
	// NoConnection offers no connection bedrocktool can make.
	NoConnection
)

// World is a world a friend is hosting.
type World struct {
	// HandleID is the Xbox Live activity handle the world is joined with.
	HandleID uuid.UUID
	// Host is the host's gamertag.
	Host      string
	OwnerXUID string
	Name      string
	// Members and MaxMembers are 0 when unknown.
	Members    int
	MaxMembers int
	Version    string
	Protocol   int
	Status     Status
}

// Address is what the address field takes to join the world.
func (w World) Address() string {
	return "xbox:" + w.HandleID.String()
}

// Title is the world's name, or the host's when it has none.
func (w World) Title() string {
	if w.Name != "" {
		return w.Name
	}
	return w.Host
}

// StatusText says why the world can't be joined; "" for one that can.
func (w World) StatusText() string {
	switch w.Status {
	case Full:
		return locale.Loc("friend_status_full", nil)
	case InviteOnly:
		return locale.Loc("friend_status_invite_only", nil)
	case Closed:
		return locale.Loc("friend_status_closed", nil)
	case OtherVersion:
		return locale.Loc("friend_status_version", locale.Strmap{"Version": w.Version, "Supported": protocol.CurrentVersion})
	case NoConnection:
		return locale.Loc("friend_status_no_connection", nil)
	}
	return ""
}

// connectable reports whether bedrocktool's proxy can dial the connection.
func connectable(c p2p.Connection) bool {
	switch c.Type {
	case p2p.ConnectionTypeSignalingOverJSONRPC, p2p.ConnectionTypeSignalingOverWebSocket:
		return true
	}
	return false
}

// FromActivities turns the activity handles the session directory returned
// into worlds, joinable ones first. Sessions of selfXUID (the account's own)
// and ones that aren't Minecraft worlds are left out. supported is the
// protocol version bedrocktool speaks.
func FromActivities(activities []mpsd.ActivityHandle, selfXUID string, supported int) []World {
	var worlds []World
	for _, a := range activities {
		var pw p2p.World
		if err := json.Unmarshal(a.CustomProperties, &pw); err != nil {
			// sessions with a connection type p2p doesn't know (LAN only)
			// fail here; they can't be joined from here anyway
			logrus.Debugf("friend world %s: %s", a.ID, err)
			continue
		}
		owner := a.OwnerXUID
		if owner == "" {
			owner = pw.OwnerID
		}
		if selfXUID != "" && owner == selfXUID {
			continue
		}
		w := World{
			HandleID:   a.ID,
			Host:       pw.HostName,
			OwnerXUID:  owner,
			Name:       strings.TrimSpace(pw.WorldName),
			Members:    pw.MemberCount,
			MaxMembers: pw.MaxMemberCount,
			Version:    pw.Version,
			Protocol:   pw.Protocol,
		}
		if w.MaxMembers == 0 && a.RelatedInfo != nil {
			w.MaxMembers = int(a.RelatedInfo.MaxMembersCount)
		}
		w.Status = status(a, pw, w, supported)
		worlds = append(worlds, w)
	}
	slices.SortStableFunc(worlds, func(a, b World) int {
		if (a.Status == Joinable) != (b.Status == Joinable) {
			if a.Status == Joinable {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Host), strings.ToLower(b.Host))
	})
	return worlds
}

func status(a mpsd.ActivityHandle, pw p2p.World, w World, supported int) Status {
	switch {
	case a.RelatedInfo != nil && a.RelatedInfo.Closed:
		return Closed
	case pw.Joinability == p2p.JoinabilityInviteOnly,
		pw.BroadcastSetting == p2p.BroadcastSettingInviteOnly,
		a.RelatedInfo != nil && a.RelatedInfo.JoinRestriction == mpsd.SessionRestrictionLocal:
		return InviteOnly
	case w.MaxMembers > 0 && w.Members >= w.MaxMembers:
		return Full
	case w.Protocol != 0 && supported != 0 && w.Protocol != supported:
		return OtherVersion
	case !slices.ContainsFunc(pw.SupportedConnections, connectable):
		return NoConnection
	}
	return Joinable
}

// CountJoinable is how many of the worlds can be joined.
func CountJoinable(worlds []World) int {
	n := 0
	for _, w := range worlds {
		if w.Status == Joinable {
			n++
		}
	}
	return n
}

// List asks Xbox Live for the worlds the account's friends are playing.
func List(ctx context.Context, account *auth.Account) ([]World, error) {
	if account == nil {
		return nil, auth.ErrNotLoggedIn
	}
	xbl, err := account.XBLClient(ctx)
	if err != nil {
		return nil, err
	}
	activities, err := xbl.MPSD().Activities(ctx, mcauth.ServiceConfigID)
	if err != nil {
		return nil, err
	}
	return FromActivities(activities, xbl.UserInfo().XUID, protocol.CurrentProtocol), nil
}

type goneError struct{}

// Error is looked up when shown, so it is in the language picked by then.
func (goneError) Error() string { return locale.Loc("friend_world_gone", nil) }

// ErrGone is returned by Find for a world that isn't open any more.
var ErrGone error = goneError{}

// Find looks the world with the handle up among the account's friends' worlds.
func Find(ctx context.Context, account *auth.Account, handleID uuid.UUID) (World, error) {
	worlds, err := List(ctx, account)
	if err != nil {
		return World{}, err
	}
	for _, w := range worlds {
		if w.HandleID == handleID {
			return w, nil
		}
	}
	return World{}, ErrGone
}

// UnjoinableError is why a friend's world can't be joined.
type UnjoinableError struct {
	World World
}

func (e *UnjoinableError) Error() string {
	return locale.Loc("friend_world_unjoinable", locale.Strmap{
		"Host":   e.World.Host,
		"Name":   e.World.Title(),
		"Reason": e.World.StatusText(),
	})
}

// PlayersText is the player count line, "" when unknown.
func (w World) PlayersText() string {
	switch {
	case w.Members > 0 && w.MaxMembers > 0:
		return locale.Loc("gui_friends_players", locale.Strmap{"Count": strconv.Itoa(w.Members), "Max": strconv.Itoa(w.MaxMembers)})
	case w.Members > 0:
		return locale.Loc("gui_friends_players_count", locale.Strmap{"Count": strconv.Itoa(w.Members)})
	}
	return ""
}
