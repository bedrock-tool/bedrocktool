module github.com/bedrock-tool/bedrocktool

go 1.20

//replace github.com/sandertv/gophertunnel => ./gophertunnel
replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.31.0-7

//replace github.com/df-mc/dragonfly => ./dragonfly
replace github.com/df-mc/dragonfly => github.com/olebeck/dragonfly v0.9.8-2

//replace gioui.org => ./gio
replace gioui.org => github.com/olebeck/gio v0.0.0-20230607184051-9ab60b08f083

require (
	gioui.org v0.0.0-20230526230622-e3ef98dda382
	gioui.org/x v0.0.0-20230523210033-8432ec5563bb
	github.com/cloudfoundry-attic/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21
	github.com/dblezek/tga v0.0.0-20150626111426-80720cbc1017
	github.com/df-mc/dragonfly v0.9.6
	github.com/df-mc/goleveldb v1.1.9
	github.com/fatih/color v1.15.0
	github.com/flytam/filenamify v1.1.3
	github.com/go-gl/mathgl v1.0.0
	github.com/google/subcommands v1.2.0
	github.com/google/uuid v1.3.0
	github.com/nicksnyder/go-i18n/v2 v2.2.1
	github.com/sanbornm/go-selfupdate v0.0.0-20210106163404-c9b625feac49
	github.com/sandertv/go-raknet v1.12.0
	github.com/sandertv/gophertunnel v1.31.0
	github.com/shirou/gopsutil/v3 v3.23.5
	github.com/sirupsen/logrus v1.9.3
	github.com/tailscale/hujson v0.0.0-20221223112325-20486734a56a
	github.com/thomaso-mirodin/intmath v0.0.0-20160323211736-5dc6d854e46e
	golang.design/x/lockfree v0.0.1
	golang.org/x/crypto v0.9.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/exp/shiny v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/oauth2 v0.8.0
	golang.org/x/term v0.8.0
	golang.org/x/text v0.9.0
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	gioui.org/cpu v0.0.0-20220412190645-f1e9e8c3b1f7 // indirect
	gioui.org/shader v1.0.6 // indirect
	git.wow.st/gmp/jni v0.0.0-20210610011705-34026c7e22d0 // indirect
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/changkun/lockfree v0.0.1 // indirect
	github.com/cloudfoundry/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/df-mc/worldupgrader v1.0.8 // indirect
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-text/typesetting v0.0.0-20230606200221-26abc51a6c27 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/kr/binarydist v0.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20230326075908-cb1d2100619a // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/image v0.7.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inconshreveable/go-update.v0 v0.0.0-20150814200126-d8b0b1d421aa // indirect
)
