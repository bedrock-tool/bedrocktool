package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/subcommands"
	buttplug "github.com/pidurentry/buttplug-go"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BpCMD struct {
	server_address string
	buttplugIP     string
	dev            buttplug.DeviceManager
}

func (*BpCMD) Name() string     { return "bpio" }
func (*BpCMD) Synopsis() string { return "buttplug.io intigration" }

func (c *BpCMD) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.server_address, "address", "", "remote server address")
	f.StringVar(&c.buttplugIP, "bpaddr", "192.168.178.50", "other address")
}
func (c *BpCMD) Usage() string {
	return c.Name() + ": " + c.Synopsis() + "\n"
}

func (c *BpCMD) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	fmt.Println("connecting to buttplug server")
	bp, err := buttplug.Dial(fmt.Sprintf("ws://%s:12345", c.buttplugIP))
	if err != nil {
		fmt.Fprintf(os.Stderr, "buttplug connect error: %s\n", err)
	}
	handler := buttplug.NewHandler(bp)
	device_manager := buttplug.NewDeviceManager(handler)
	c.dev = device_manager

	scan := c.dev.Scan(30 * time.Second)
	fmt.Println("Scanning for devices...")
	go func() {
		scan.Wait()
		fmt.Println("Stopped scanning")
		fmt.Println("Found: (")
		for _, d := range c.dev.Devices() {
			fmt.Printf("\t%s\n", d.DeviceName())
		}
		fmt.Println(")")
	}()

	address, _, err := server_input(c.server_address)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		return 1
	}

	listener, clientConn, serverConn, err := create_proxy(ctx, address)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer listener.Close()

	println("Connected")

	type vib struct {
		v   buttplug.Vibrate
		end time.Time
	}

	vibrates := make(map[string]vib)
	vib_mutex := &sync.Mutex{}

	go func() {
		for {
			now := time.Now()
			var to_clear []string
			vib_mutex.Lock()
			for name, v := range vibrates {
				if now.After(v.end) {
					fmt.Println("stopping vibrate")
					v.v.Stop()
					to_clear = append(to_clear, name)
				}
			}
			for _, v := range to_clear {
				delete(vibrates, v)
			}
			vib_mutex.Unlock()
			time.Sleep(1 * time.Second)
		}
	}()

	vibrate_ms := func(v buttplug.Vibrate, ms int) {
		vib_mutex.Lock()
		var name string = string(v.DeviceName())
		t := time.Now()
		if v_e, ok := vibrates[name]; ok {
			t = v_e.end
		}
		vibrates[name] = vib{
			v:   v,
			end: t.Add(time.Duration(ms) * time.Millisecond),
		}
		vib_mutex.Unlock()
	}

	errs := make(chan error, 2)
	go func() { // server -> client
		defer serverConn.Close()
		defer listener.Disconnect(clientConn, "connection lost")
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(clientConn, disconnect.Error())
				}
				return
			}

			if err = clientConn.WritePacket(pk); err != nil {
				return
			}
		}
	}()

	go func() { // client -> server
		for {
			pk, err := clientConn.ReadPacket()
			if err != nil {
				return
			}

			switch pk := pk.(type) {
			case *packet.Animate:
				if pk.ActionType == packet.AnimateActionSwingArm {
					devs := device_manager.Vibrators()
					fmt.Printf("vibrating %d devices\n", len(devs))
					for _, v := range devs {
						vibrate_ms(v, 1000)
					}
				}
			}

			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(clientConn, disconnect.Error())
				}
				return
			}
		}
	}()

	for {
		select {
		case err := <-errs:
			fmt.Fprintln(os.Stderr, err)
			return 1
		case <-ctx.Done():
			return 0
		}
	}
}

func init() {
	register_command(&BpCMD{})
}
