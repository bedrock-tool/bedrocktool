package worldstate

import (
	"slices"
	"strings"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// seatLift is how far above a definition's seat the game puts a rider's
// seat offset (BDS sends 1.32 for a seat at 0.1), which is what Hive sends.
const seatLift = 1.22

// mountSeatsLocked is, for each custom entity type something rides, where
// its riders sit, from the seat offsets the server gave them (the most
// riders any one entity of the type had). Called with stateLock held.
func (w *World) mountSeatsLocked() map[string][]mgl32.Vec3 {
	out := make(map[string][]mgl32.Vec3)
	for ridden, riders := range w.memState.entityLinks {
		if len(riders) == 0 {
			continue
		}
		mount := w.memState.entities[w.memState.uniqueIDsToRuntimeIDs[ridden]]
		if mount == nil || mount.UniqueID != ridden || strings.HasPrefix(mount.EntityType, "minecraft:") {
			continue
		}
		scale := float32(1)
		if s, ok := mount.Metadata[protocol.EntityDataKeyScale].(float32); ok && s > 0 {
			scale = s
		}
		ids := make([]int64, 0, len(riders))
		for id := range riders {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		var seats []mgl32.Vec3
		for _, id := range ids {
			rider := w.memState.entities[w.memState.uniqueIDsToRuntimeIDs[id]]
			if rider == nil || rider.UniqueID != id {
				continue // not saved (a player left out)
			}
			off, _ := rider.Metadata[protocol.EntityDataKeySeatOffset].(mgl32.Vec3)
			// the game scales seats with the mount
			seats = append(seats, mgl32.Vec3{off.X() / scale, (off.Y() - seatLift) / scale, off.Z() / scale})
		}
		if len(seats) > len(out[mount.EntityType]) {
			out[mount.EntityType] = seats
		}
	}
	return out
}

// rideable is the minecraft:rideable component with these seats.
func rideable(seats []mgl32.Vec3) map[string]any {
	list := make([]any, len(seats))
	for i, s := range seats {
		list[i] = map[string]any{"position": []float32{s.X(), s.Y(), s.Z()}}
	}
	return map[string]any{
		"seat_count":              len(seats),
		"seats":                   list,
		"pull_in_entities":        false,
		"crouching_skip_interact": true,
	}
}
