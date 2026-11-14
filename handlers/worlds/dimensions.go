package worlds

import (
	"fmt"
	"os"
	"sync"

	"github.com/bedrock-tool/bedrocktool/handlers/worlds/worldstate"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sirupsen/logrus"
)

// dimensionMerge is the world that the worlds after it are merged into, as
// custom dimensions (-merge-dimensions).
type dimensionMerge struct {
	mu sync.Mutex
	// lastSave is closed when the last world given to be saved is done.
	lastSave chan struct{}

	mainFolder string
	mainTitle  string
	mainSpawn  mgl32.Vec3
	dims       []worldstate.MergedDimension
	taken      map[string]bool
}

// mergeSavedWorld merges a saved world into the first saved world, as a
// custom dimension. The first world is kept as it is (merged is false), to
// become the main one.
func (w *worldsHandler) mergeSavedWorld(m *dimensionMerge, worldState *worldstate.World) (merged bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mainFolder == "" {
		m.mainFolder = worldState.Folder
		m.mainTitle = worldState.Name
		m.mainSpawn = worldState.Spawn()
		return false, nil
	}

	dim := worldstate.MergedDimension{
		ID:         worldstate.FirstCustomDimensionID + len(m.dims),
		Identifier: worldstate.DimensionIdentifier(worldState.Name, m.taken),
		Title:      worldState.Name,
		Spawn:      worldState.Spawn(),
		Range:      worldstate.TargetRange(worldState.Range()),
	}
	srcDim, _ := world.DimensionID(worldState.Dimension())

	w.log.Info(locale.Loc("worlds_merging_dimension", locale.Strmap{
		"Name": worldState.Name, "Main": m.mainTitle, "Dimension": dim.Identifier,
	}))
	messages.SendEvent(&messages.EventProcessingWorldUpdate{
		WorldName: worldState.Name,
		State:     "Merging into " + m.mainTitle,
	})

	stats, err := worldstate.MergeWorld(m.mainFolder, worldState.Folder, srcDim, worldState.Range(), dim)
	if err != nil {
		return false, err
	}
	if err := worldstate.MergePacks(m.mainFolder, worldState.Folder); err != nil {
		return false, err
	}
	m.taken[dim.Identifier] = true
	m.dims = append(m.dims, dim)

	err = worldstate.WriteDimensionPack(m.mainFolder, utils.MakeValidFilename(w.serverState.serverName+" "+m.mainTitle), m.mainTitle, m.mainSpawn, m.dims)
	if err != nil {
		return true, err
	}

	filename := m.mainFolder + ".mcworld"
	if err := zipWorld(m.mainFolder, filename); err != nil {
		return true, err
	}
	// the world now lives on in the main one
	if err := os.RemoveAll(worldState.Folder); err != nil {
		w.log.Warn(err)
	}

	w.log.WithFields(logrus.Fields{
		"Chunks":   stats.Chunks,
		"Entities": stats.Entities,
		"Maps":     stats.Maps,
	}).Info(locale.Loc("worlds_merged_dimension", locale.Strmap{
		"Name":      worldState.Name,
		"Dimension": dim.Identifier,
		"File":      filename,
		"Command":   fmt.Sprintf("/%s %s", worldstate.DimensionCommand, dim.ShortName()),
	}))
	if stats.RenamedIDs > 0 {
		w.log.Debugf("%d entities got new unique ids", stats.RenamedIDs)
	}
	if len(m.dims) == 1 {
		w.log.Info(locale.Loc("worlds_merge_requirements", locale.Strmap{"Command": "/" + worldstate.DimensionCommand}))
	}

	total, _ := worldState.EntityCounts(w.serverState.getEntityRenderDistance())
	messages.SendEvent(&messages.EventFinishedSavingWorld{
		WorldName: worldState.Name,
		Filepath:  filename,
		Chunks:    len(worldState.StoredChunks),
		Entities:  total,
	})
	return true, nil
}
