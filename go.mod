module github.com/bedrock-tool/bedrocktool

go 1.19

//replace github.com/sandertv/gophertunnel => ./gophertunnel
replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.26.2

//replace github.com/df-mc/dragonfly => ./dragonfly
replace github.com/df-mc/dragonfly => github.com/olebeck/dragonfly v0.9.1-3

require (
	github.com/cloudfoundry-attic/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21
	github.com/df-mc/dragonfly v0.9.1
	github.com/df-mc/goleveldb v1.1.9
	github.com/fatih/color v1.13.0
	github.com/flytam/filenamify v1.1.2
	github.com/go-gl/mathgl v1.0.0
	github.com/google/subcommands v1.2.0
	github.com/google/uuid v1.3.0
	github.com/jinzhu/copier v0.3.5
	github.com/miekg/dns v1.1.50
	github.com/nicksnyder/go-i18n/v2 v2.2.1
	github.com/sanbornm/go-selfupdate v0.0.0-20210106163404-c9b625feac49
	github.com/sandertv/gophertunnel v1.26.0
	github.com/sirupsen/logrus v1.9.0
	golang.design/x/lockfree v0.0.1
	golang.org/x/exp v0.0.0-20230113213754-f9f960f08ad4
	golang.org/x/image v0.3.0
	golang.org/x/oauth2 v0.4.0
	golang.org/x/text v0.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/changkun/lockfree v0.0.1 // indirect
	github.com/cloudfoundry/jibber_jabber v0.0.0-20151120183258-bcc4c8345a21 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.15.14 // indirect
	github.com/kr/binarydist v0.1.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/sandertv/go-raknet v1.12.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	golang.org/x/tools v0.5.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/inconshreveable/go-update.v0 v0.0.0-20150814200126-d8b0b1d421aa // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)
