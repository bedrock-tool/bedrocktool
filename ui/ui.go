package ui

import (
	"context"
)

type UI interface {
	Init() error
	Start(context.Context, context.CancelCauseFunc) error
}
