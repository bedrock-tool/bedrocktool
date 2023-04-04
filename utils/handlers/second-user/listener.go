package seconduser

import "github.com/df-mc/dragonfly/server/session"

type fwdlistener struct {
	Conn chan session.Conn
}

func (l *fwdlistener) Accept() (session.Conn, error) {
	return <-l.Conn, nil
}

func (l *fwdlistener) Disconnect(conn session.Conn, reason string) error {
	return conn.Close()
}

func (l *fwdlistener) Close() error {
	return nil
}
