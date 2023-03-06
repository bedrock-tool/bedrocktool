package gui

import "context"

type UI interface {
	Init()
	SetOptions() bool
	Execute(context.Context) error
}
