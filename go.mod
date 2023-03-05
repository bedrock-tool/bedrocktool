module github.com/bedrock-tool/bedrocktool

go 1.19

//replace github.com/sandertv/gophertunnel => ./gophertunnel
replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.27.4-1

//replace github.com/df-mc/dragonfly => ./dragonfly
replace github.com/df-mc/dragonfly => github.com/olebeck/dragonfly v0.9.3-1

require (
	fyne.io/fyne/v2 v2.3.1
	github.com/cloudfoundry-attic/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21
	github.com/df-mc/dragonfly v0.9.1
	github.com/df-mc/goleveldb v1.1.9
	github.com/fatih/color v1.14.1
	github.com/flytam/filenamify v1.1.2
	github.com/go-gl/mathgl v1.0.0
	github.com/google/subcommands v1.2.0
	github.com/google/uuid v1.3.0
	github.com/jinzhu/copier v0.3.5
	github.com/miekg/dns v1.1.50
	github.com/nicksnyder/go-i18n/v2 v2.2.1
	github.com/sanbornm/go-selfupdate v0.0.0-20210106163404-c9b625feac49
	github.com/sandertv/gophertunnel v1.27.4
	github.com/sirupsen/logrus v1.9.0
	golang.design/x/lockfree v0.0.1
	golang.org/x/crypto v0.5.0
	golang.org/x/exp v0.0.0-20230206171751-46f607a40771
	golang.org/x/image v0.5.0
	golang.org/x/oauth2 v0.4.0
	golang.org/x/text v0.7.0
	gopkg.in/square/go-jose.v2 v2.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	fyne.io/systray v1.10.1-0.20230207085535-4a244dbb9d03 // indirect
	github.com/benoitkugler/textlayout v0.3.0 // indirect
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/changkun/lockfree v0.0.1 // indirect
	github.com/cloudfoundry/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/fredbi/uri v0.1.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/fyne-io/gl-js v0.0.0-20220119005834-d2da28d9ccfe // indirect
	github.com/fyne-io/glfw-js v0.0.0-20220120001248-ee7290d23504 // indirect
	github.com/fyne-io/image v0.0.0-20220602074514-4956b0afb3d2 // indirect
	github.com/go-gl/gl v0.0.0-20211210172815-726fda9656d6 // indirect
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20221017161538-93cebf72946b // indirect
	github.com/go-text/typesetting v0.0.0-20221212183139-1eb938670a1f // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/goki/freetype v0.0.0-20220119013949-7a161fd3728c // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/jsummers/gobmp v0.0.0-20151104160322-e2ba15ffa76e // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/kr/binarydist v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sandertv/go-raknet v1.12.0 // indirect
	github.com/srwiley/oksvg v0.0.0-20220731023508-a61f04f16b76 // indirect
	github.com/srwiley/rasterx v0.0.0-20210519020934-456a8d69b780 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	github.com/tevino/abool v1.2.0 // indirect
	github.com/yuin/goldmark v1.4.13 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/mobile v0.0.0-20211207041440-4e6c2922fdee // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/tools v0.5.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inconshreveable/go-update.v0 v0.0.0-20150814200126-d8b0b1d421aa // indirect
	honnef.co/go/js/dom v0.0.0-20210725211120-f030747120f2 // indirect
)
