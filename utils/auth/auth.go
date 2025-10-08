package auth

import (
	"context"
	"errors"
	"os"
	"sync/atomic"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/auth/xbox"
	"github.com/sirupsen/logrus"
)

var ErrNotLoggedIn = errors.New("not Logged In")

var defaultDeviceType = &xbox.DeviceTypeAndroid

type authSrv struct {
	log     *logrus.Entry
	env     string
	handler xbox.MSAuthHandler
	account atomic.Pointer[Account]

	authCtxCancel atomic.Pointer[context.CancelFunc]
}

var Auth *authSrv = &authSrv{
	log: logrus.WithField("part", "Auth"),
}

func (a *authSrv) SetEnv(env string) {
	a.env = env
}

// reads token from storage if there is one
func (a *authSrv) LoadAccount(name string) (err error) {
	tokenInfo, err := readAuth[tokenInfo](tokenFileName(name))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errors.ErrUnsupported) {
		return nil
	}
	if err != nil {
		return err
	}
	a.account.Store(&Account{
		token: tokenInfo,
		name:  name,
		env:   a.env,
	})
	return nil
}

// if the user is currently logged in or not
func (a *authSrv) LoggedIn() bool {
	return a.account.Load() != nil
}

// performs microsoft login using the handler passed
func (a *authSrv) SetHandler(handler xbox.MSAuthHandler) (err error) {
	a.handler = handler
	return nil
}

func (a *authSrv) Login(ctx context.Context, deviceType *xbox.DeviceType, name string) (err error) {
	liveToken, err := xbox.RequestLiveTokenWriter(ctx, deviceType, a.handler)
	if err != nil {
		return err
	}
	a.account.Store(&Account{
		token: &tokenInfo{
			Token:      liveToken,
			DeviceType: deviceType.DeviceType,
		},
		name: name,
		env:  a.env,
	})
	if err = writeAuth(tokenFileName(name), *liveToken); err != nil {
		return err
	}
	return nil
}

func (a *authSrv) Logout() {
	acc := a.account.Swap(nil)
	os.Remove(tokenFileName(acc.name))
	os.Remove(chainFileName(acc.name))
}

func (a *authSrv) Account() *Account {
	return a.account.Load()
}

func (a *authSrv) RequestLogin(name string) error {
	ctx, cancel := context.WithCancel(context.Background())
	a.authCtxCancel.Store(&cancel)
	defer cancel()
	err := a.Login(ctx, defaultDeviceType, name)
	messages.SendEvent(&messages.EventAuthFinished{
		Error: err,
	})
	return err
}

func (a *authSrv) CancelLogin() {
	cancel := a.authCtxCancel.Swap(nil)
	if cancel != nil {
		(*cancel)()
	}
}
