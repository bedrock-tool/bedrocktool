package utils

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var ErrorHandler = func(err error) {
	logrus.Fatal(err)
}

func RecoverCall(f func() error) (err error) {
	defer func() {
		if errr, ok := recover().(error); ok {
			err = errr
		}
	}()
	err = f()
	return
}

// RecoverGo runs f, handing a panic in it to ErrorHandler instead of
// letting it end the program. For goroutines: a panic in one that isn't
// recovered kills the whole app without a word in the log.
func RecoverGo(f func()) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
			ErrorHandler(err)
		}
	}()
	f()
}
