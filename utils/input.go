package utils

import (
	"context"
	"io"

	"github.com/sirupsen/logrus"

	"github.com/chzyer/readline"
)

func UserInput(ctx context.Context, q string, validator func(string) bool) (string, bool) {
	inst, err := readline.New(q)
	if err != nil {
		panic(err)
	}
	line, err := inst.Readline()
	switch {
	case err == io.EOF:
		return "", true
	case err == readline.ErrInterrupt:
		return "", true
	case err != nil:
		logrus.Error(err)
		return "", true
	default:
		return line, false
	}
}
