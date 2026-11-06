module github.com/bedrock-tool/bedrocktool

go 1.26.0

// that repo has a bad go.mod so need to force the old version
replace github.com/brentp/intintmap => github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9

require (
	gioui.org v0.10.3
	gioui.org/x v0.10.3
	git.wow.st/gmp/jni v0.0.0-20260821030830-2b2c1e0034f1
	github.com/OneOfOne/xxhash v1.2.8
	github.com/chzyer/readline v1.5.1
	github.com/cloudfoundry/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21
	github.com/coder/websocket v1.8.15
	github.com/dblezek/tga v0.0.0-20150626111426-80720cbc1017
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/df-mc/dragonfly v0.11.6
	github.com/df-mc/go-nethernet v1.0.27
	github.com/df-mc/go-playfab/v2 v2.0.3
	github.com/df-mc/go-xsapi v1.0.1
	github.com/df-mc/go-xsapi/v2 v2.0.3
	github.com/df-mc/goleveldb v1.1.9
	github.com/dop251/goja v0.0.0-20261105002551-2413f2132ac8
	github.com/dop251/goja_nodejs v0.0.0-20260918173711-b481721df8a2
	github.com/fatih/color v1.19.0
	github.com/gioui-plugins/gio-plugins v0.9.2
	github.com/go-gl/mathgl v1.2.0
	github.com/go-jose/go-jose/v4 v4.1.9
	github.com/google/uuid v1.6.0
	github.com/klauspost/compress v1.20.3
	github.com/minio/selfupdate v0.6.0
	github.com/nicksnyder/go-i18n/v2 v2.7.0
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sandertv/go-raknet v1.15.2
	github.com/sandertv/gophertunnel v1.63.0
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/sirupsen/logrus v1.10.2
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f
	github.com/zaataylor/cartesian v0.0.0-20221028053253-3b3244d82727
	golang.org/x/exp v0.0.0-20261106112924-d498a4c2cc9a
	golang.org/x/exp/shiny v0.0.0-20261106112924-d498a4c2cc9a
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.43.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	aead.dev/minisign v0.3.0 // indirect
	gioui.org/shader v1.0.9 // indirect
	github.com/brentp/intintmap v0.0.0-20251106190759-56907b1f8479 // indirect
	github.com/coreos/go-oidc/v3 v3.21.0 // indirect
	github.com/df-mc/jsonc v1.0.5 // indirect
	github.com/df-mc/worldupgrader v1.0.22 // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/dlclark/regexp2/v2 v2.12.0 // indirect
	github.com/ftrvxmtrx/tga v0.0.0-20150524081124-bd8e8d5be13a // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/godbus/dbus/v5 v5.2.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/pprof v0.0.0-20261105003125-4bf881d7fb0b // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/pion/datachannel v1.6.4 // indirect
	github.com/pion/dtls/v3 v3.1.15 // indirect
	github.com/pion/ice/v4 v4.4.8 // indirect
	github.com/pion/interceptor v0.1.51 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/mdns/v2 v2.2.2 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.20 // indirect
	github.com/pion/rtp v1.10.6 // indirect
	github.com/pion/sctp v1.12.1 // indirect
	github.com/pion/sdp/v3 v3.0.20 // indirect
	github.com/pion/srtp/v3 v3.1.3 // indirect
	github.com/pion/stun/v3 v3.0.0 // indirect
	github.com/pion/stun/v4 v4.0.1 // indirect
	github.com/pion/transport/v3 v3.0.7 // indirect
	github.com/pion/transport/v5 v5.1.4 // indirect
	github.com/pion/turn/v4 v4.0.0 // indirect
	github.com/pion/turn/v5 v5.1.3 // indirect
	github.com/pion/webrtc/v4 v4.2.26 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/time v0.16.0 // indirect
)
