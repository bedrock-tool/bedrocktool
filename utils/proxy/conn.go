package proxy

import (
	"context"
	"errors"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

func connectServer(ctx context.Context, address string, ClientData *login.ClientData, wantPacks bool, packetFunc PacketFunc, tokenSource oauth2.TokenSource) (serverConn *minecraft.Conn, err error) {
	cd := login.ClientData{}
	if ClientData != nil {
		cd = *ClientData
	}

	logrus.Info(locale.Loc("connecting", locale.Strmap{"Address": address}))
	serverConn, err = minecraft.Dialer{
		TokenSource: tokenSource,
		ClientData:  cd,
		PacketFunc:  packetFunc,
		DownloadResourcePack: func(id uuid.UUID, version string, current int, total int) bool {
			return wantPacks
		},
	}.DialContext(ctx, "raknet", address)
	if err != nil {
		return serverConn, err
	}

	logrus.Debug(locale.Loc("connected", nil))
	return serverConn, nil
}

func spawnConn(ctx context.Context, clientConn minecraft.IConn, serverConn minecraft.IConn, gd minecraft.GameData) error {
	wg := sync.WaitGroup{}
	errs := make(chan error, 2)
	if clientConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- clientConn.StartGame(gd)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- serverConn.DoSpawn()
	}()

	wg.Wait()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				return errors.New(locale.Loc("failed_start_game", locale.Strmap{"Err": err}))
			}
		case <-ctx.Done():
			return errors.New(locale.Loc("connection_cancelled", nil))
		default:
		}
	}
	return nil
}
