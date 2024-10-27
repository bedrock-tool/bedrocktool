module github.com/bedrock-tool/bedrocktool

go 1.23

toolchain go1.23.1

// that repo has a bad go.mod so need to force the old version
replace github.com/brentp/intintmap => github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9

require (
	gioui.org v0.7.1
	gioui.org/x v0.7.1
	github.com/OneOfOne/xxhash v1.2.8
	github.com/cloudfoundry/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21
	github.com/dblezek/tga v0.0.0-20150626111426-80720cbc1017
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/df-mc/dragonfly v0.9.18
	github.com/df-mc/goleveldb v1.1.9
	github.com/dop251/goja v0.0.0-20241024094426-79f3a7efcdbd
	github.com/dop251/goja_nodejs v0.0.0-20240728170619-29b559befffc
	github.com/fatih/color v1.18.0
	github.com/flytam/filenamify v1.2.0
	github.com/go-gl/mathgl v1.1.0
	github.com/go-jose/go-jose/v3 v3.0.3
	github.com/google/uuid v1.6.0
	github.com/klauspost/compress v1.17.11
	github.com/minio/selfupdate v0.6.0
	github.com/nicksnyder/go-i18n/v2 v2.4.1
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sandertv/go-raknet v1.14.2
	github.com/sandertv/gophertunnel v1.41.1
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/sirupsen/logrus v1.9.3
	github.com/tailscale/hujson v0.0.0-20241010212012-29efb4a0184b
	github.com/thomaso-mirodin/intmath v0.0.0-20160323211736-5dc6d854e46e
	golang.design/x/lockfree v0.0.1
	golang.org/x/crypto v0.28.0
	golang.org/x/exp v0.0.0-20241009180824-f66d83c29e7c
	golang.org/x/exp/shiny v0.0.0-20241009180824-f66d83c29e7c
	golang.org/x/oauth2 v0.23.0
	golang.org/x/sys v0.26.0
	golang.org/x/term v0.25.0
	golang.org/x/text v0.19.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	aead.dev/minisign v0.2.0 // indirect
	gioui.org/cpu v0.0.0-20210817075930-8d6a761490d2 // indirect
	gioui.org/shader v1.0.8 // indirect
	git.wow.st/gmp/jni v0.0.0-20210610011705-34026c7e22d0 // indirect
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/changkun/lockfree v0.0.1 // indirect
	github.com/df-mc/worldupgrader v1.0.17 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/ftrvxmtrx/tga v0.0.0-20150524081124-bd8e8d5be13a // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/go-text/typesetting v0.1.1 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/image v0.21.0 // indirect
	golang.org/x/net v0.30.0 // indirect
)
