package gui

import "context"

type UI interface {
	Init()
	SetOptions()
	Execute(context.Context) error
}
