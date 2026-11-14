package auth

import (
	"context"
	"errors"
	"os"
	"sync/atomic"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sirupsen/logrus"
)

var ErrNotLoggedIn = errors.New("not Logged In")

var defaultDeviceType = &auth.AndroidConfig

type authSrv struct {
	log     *logrus.Entry
	env     string
	handler auth.AuthCodeHandler
	account atomic.Pointer[Account]

	authCtxCancel atomic.Pointer[context.CancelFunc]
}

var Auth *authSrv = &authSrv{
	log: logrus.WithField("part", "Auth"),
}

func (a *authSrv) SetEnv(env string) {
	a.env = env
}

func (a *authSrv) setAccount(acc *Account) *Account {
	prevAcc := a.account.Swap(acc)
	if prevAcc != nil {
		prevAcc.Close()
	}
	return prevAcc
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
	a.setAccount(&Account{
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
func (a *authSrv) SetHandler(handler auth.AuthCodeHandler) (err error) {
	if handler == nil {
		handler = auth.CliAuthCodeHandler{Writer: os.Stdout}
	}
	a.handler = handler
	return nil
}

func (a *authSrv) Login(ctx context.Context, conf *auth.Config, name string) (err error) {
	if conf == nil {
		conf = &auth.AndroidConfig
	}
	liveToken, err := conf.RequestLiveTokenContext(ctx, a.handler)
	if err != nil {
		return err
	}
	tokenInfo := tokenInfo{
		Token:    liveToken,
		ClientID: conf.ClientID,
	}
	if err = writeAuth(tokenFileName(name), tokenInfo); err != nil {
		return err
	}
	a.setAccount(&Account{
		token: &tokenInfo,
		name:  name,
		env:   a.env,
	})
	return nil
}

// forget logs out acc if it is still the current account: its saved
// tokens are deleted and LoggedIn reports false, so the next session
// asks for a new sign-in.
func (a *authSrv) forget(acc *Account) {
	if !a.account.CompareAndSwap(acc, nil) {
		return
	}
	a.log.Warn("Microsoft no longer accepts the saved login, removed it. Sign in again.")
	os.Remove(tokenFileName(acc.name))
	os.Remove(chainFileName(acc.name))
}

func (a *authSrv) Logout() {
	prevAcc := a.setAccount(nil)
	os.Remove(tokenFileName(prevAcc.name))
	os.Remove(chainFileName(prevAcc.name))
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
