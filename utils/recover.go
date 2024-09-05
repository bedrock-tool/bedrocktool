package utils

import "github.com/sirupsen/logrus"

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
