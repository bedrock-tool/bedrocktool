module github.com/bedrock-tool/bedrocktool

go 1.20

//replace github.com/sandertv/gophertunnel => ./gophertunnel
replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.29.0-1

//replace github.com/df-mc/dragonfly => ./dragonfly
replace github.com/df-mc/dragonfly => github.com/olebeck/dragonfly v0.9.5-1

//replace gioui.org => ./gio
replace gioui.org => github.com/olebeck/gio v0.0.0-20230427194143-c9c9d8bc704d

require (
	gioui.org v0.0.0-20230427133431-816bda7ac7bd
	gioui.org/x v0.0.0-20230426160849-752f112c7a59
	github.com/cloudfoundry-attic/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21
	github.com/df-mc/dragonfly v0.9.4
	github.com/df-mc/goleveldb v1.1.9
	github.com/fatih/color v1.15.0
	github.com/flytam/filenamify v1.1.2
	github.com/go-gl/mathgl v1.0.0
	github.com/google/subcommands v1.2.0
	github.com/google/uuid v1.3.0
	github.com/jinzhu/copier v0.3.5
	github.com/miekg/dns v1.1.53
	github.com/nicksnyder/go-i18n/v2 v2.2.1
	github.com/repeale/fp-go v0.11.1
	github.com/sanbornm/go-selfupdate v0.0.0-20210106163404-c9b625feac49
	github.com/sandertv/go-raknet v1.12.0
	github.com/sandertv/gophertunnel v1.29.0
	github.com/shirou/gopsutil/v3 v3.23.3
	github.com/sirupsen/logrus v1.9.0
	golang.design/x/lockfree v0.0.1
	golang.org/x/crypto v0.8.0
	golang.org/x/exp v0.0.0-20230425010034-47ecfdc1ba53
	golang.org/x/exp/shiny v0.0.0-20220827204233-334a2380cb91
	golang.org/x/oauth2 v0.7.0
	golang.org/x/term v0.7.0
	golang.org/x/text v0.9.0
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	gioui.org/cpu v0.0.0-20210817075930-8d6a761490d2 // indirect
	gioui.org/shader v1.0.6 // indirect
	git.wow.st/gmp/jni v0.0.0-20210610011705-34026c7e22d0 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/changkun/lockfree v0.0.1 // indirect
	github.com/cloudfoundry/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/df-mc/worldupgrader v1.0.6 // indirect
	github.com/dlclark/regexp2 v1.9.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-text/typesetting v0.0.0-20230413204129-b4f0492bf7ae // indirect
	github.com/gofiber/fiber/v2 v2.45.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.16.3 // indirect
	github.com/kr/binarydist v0.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	github.com/savsgio/dictpool v0.0.0-20221023140959-7bf2e61cea94 // indirect
	github.com/savsgio/gotils v0.0.0-20230208104028-c358bd845dee // indirect
	github.com/shoenig/go-m1cpu v0.1.4 // indirect
	github.com/tinylib/msgp v1.1.8 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.47.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/image v0.7.0 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inconshreveable/go-update.v0 v0.0.0-20150814200126-d8b0b1d421aa // indirect
)
