package ui

import (
	"context"
)

type UI interface {
	Init() bool
	Start(context.Context, context.CancelCauseFunc) error
}
