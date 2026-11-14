package worlds

import (
	"maps"
	"regexp"
	"strings"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

var colorCodes = regexp.MustCompile("§.")

// isHiveServer says whether we're downloading from The Hive, by its address
// (geo.hivebedrock.network, ca.hivebedrock.network, play.playhive.com...) or
// the name its ping gives.
func isHiveServer(address, serverName string) bool {
	address = strings.ToLower(address)
	if strings.Contains(address, "hivebedrock.network") || strings.Contains(address, "playhive.com") {
		return true
	}
	name := strings.ToLower(colorCodes.ReplaceAllString(serverName, ""))
	return strings.Contains(name, "the hive") || strings.Contains(name, "hivebedrock")
}

// hasHivePacks says whether the server sent Hive's own resource packs
// ("HiveCommon", "The Hive: Hub", "Hive UI"...). It finds Hive in replays,
// which have no address, and behind proxies with their own.
func hasHivePacks(packs []resource.Pack) bool {
	for _, p := range packs {
		name := strings.ToLower(strings.TrimSpace(colorCodes.ReplaceAllString(p.Name(), "")))
		if strings.HasPrefix(name, "hive") || strings.HasPrefix(name, "the hive") {
			return true
		}
	}
	return false
}

// isNPCPlayer says whether a listed player is one the server made up (Hive's
// NPCs and adverts) rather than someone playing: it has no XUID. A player
// never listed counts as real.
func isNPCPlayer(xuids map[uuid.UUID]string, id uuid.UUID) bool {
	x, ok := xuids[id]
	return ok && (x == "" || x == "0")
}

// keepPlayer is which players a save keeps: all of them with Players on;
// otherwise on Hive only its NPC/advert players, and elsewhere none.
// A script's PlayerAdd callback decides first for the players it answered.
func (w *worldsHandler) keepPlayer() func(uuid.UUID) bool {
	base := w.defaultKeepPlayer()
	if len(w.serverState.scriptPlayerKeep) == 0 {
		return base
	}
	decided := maps.Clone(w.serverState.scriptPlayerKeep)
	return func(id uuid.UUID) bool {
		if keep, ok := decided[id]; ok {
			return keep
		}
		return base != nil && base(id)
	}
}

func (w *worldsHandler) defaultKeepPlayer() func(uuid.UUID) bool {
	if w.settings.Players {
		return func(uuid.UUID) bool { return true }
	}
	if !w.serverState.isHive {
		return nil
	}
	xuids := maps.Clone(w.serverState.playerXUIDs)
	return func(id uuid.UUID) bool { return isNPCPlayer(xuids, id) }
}

// onMapSeen asks the server for a map an item frame or container holds,
// once per session, unless its pixels already arrived.
func (w *worldsHandler) onMapSeen(id int64) {
	if id == ViewMapID {
		return
	}
	w.serverState.mapsMu.Lock()
	_, have := w.serverState.knownMaps[id]
	_, asked := w.serverState.requestedMaps[id]
	if !have && !asked {
		w.serverState.requestedMaps[id] = struct{}{}
	}
	w.serverState.mapsMu.Unlock()
	if have || asked || w.session.Server == nil {
		return
	}
	if err := w.session.Server.WritePacket(&packet.MapInfoRequest{MapID: id}); err != nil {
		logrus.Debugf("MapInfoRequest %d: %s", id, err)
	}
}

// storeKnownMap keeps a map's pixels for the whole session, so a later world
// holding the same map gets them too.
func (w *worldsHandler) storeKnownMap(pk *packet.ClientBoundMapItemData) {
	w.serverState.mapsMu.Lock()
	defer w.serverState.mapsMu.Unlock()
	w.serverState.knownMaps[pk.MapID] = worldstate.ApplyMapPacket(w.serverState.knownMaps[pk.MapID], pk)
}

func (w *worldsHandler) knownMapsSnapshot() map[int64]*worldstate.Map {
	w.serverState.mapsMu.Lock()
	defer w.serverState.mapsMu.Unlock()
	return maps.Clone(w.serverState.knownMaps)
}
