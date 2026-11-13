package entity

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestCustomNameVisible(t *testing.T) {
	for _, c := range []struct {
		v    any
		want bool
	}{{byte(1), true}, {byte(0), false}, {int32(1), true}, {int32(0), false}} {
		meta := protocol.NewEntityMetadata()
		meta[protocol.EntityDataKeyAlwaysShowNameTag] = c.v
		e := &Entity{Metadata: meta}
		if got := e.ToChunkEntity(nil).Data["CustomNameVisible"]; got != c.want {
			t.Errorf("AlwaysShowNameTag %T(%v): CustomNameVisible %v, want %v", c.v, c.v, got, c.want)
		}
	}
}
