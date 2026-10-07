package behaviourpack

import (
	"fmt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"strings"
	"testing"
)

func TestAddBlockPanicIncludesStageAndIdentifier(t *testing.T) {
	defer func() {
		reason := fmt.Sprint(recover())
		if !strings.Contains(reason, "custom-block-export") || !strings.Contains(reason, "test:broken") {
			t.Fatalf("panic missing actionable context: %s", reason)
		}
	}()
	New("test").AddBlock(protocol.BlockEntry{Name: "test:broken", Properties: map[string]any{
		"properties": []any{map[string]any{"name": "test:state", "enum": int32(1)}},
	}})
	t.Fatal("expected unsupported enum to panic")
}

func TestParseBlockTagsWithoutComponents(t *testing.T) {
	block, _ := parseBlock(protocol.BlockEntry{Name: "test:tagged", Properties: map[string]any{"blockTags": []any{"test:tag"}}})
	tags, ok := block.Components["minecraft:tags"].([]string)
	if !ok || len(tags) != 1 || tags[0] != "test:tag" {
		t.Fatalf("missing block tags: %#v", block.Components)
	}
}
