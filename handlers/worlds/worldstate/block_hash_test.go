package worldstate

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sirupsen/logrus"
	"testing"
)

func TestUnknownUpdateHashDoesNotResolveToRuntimeZero(t *testing.T) {
	br := world.NewBlockRegistry()
	br.Finalize()
	w := &World{BlockRegistry: br, UseHashedRids: true, log: logrus.NewEntry(logrus.New())}
	_, _, _, found := w.BlockByID(4171510507)
	if w.ExportError() == nil {
		t.Fatal("unknown hash did not block final export")
	}
	if found {
		t.Fatal("unknown hash silently resolved to runtime zero")
	}
}
