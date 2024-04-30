package ui

import (
	"context"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type UI interface {
	Init() bool
	Start(context.Context, context.CancelCauseFunc) error
	messages.Handler
}
