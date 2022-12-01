module github.com/bedrock-tool/bedrocktool

go 1.19

//replace github.com/sandertv/gophertunnel => ./gophertunnel
replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.26.1

//replace github.com/df-mc/dragonfly => ./dragonfly
replace github.com/df-mc/dragonfly => github.com/olebeck/dragonfly v0.8.10-1

require (
	github.com/df-mc/dragonfly v0.8.5
	github.com/df-mc/goleveldb v1.1.9
	github.com/fatih/color v1.13.0
	github.com/flytam/filenamify v1.1.1
	github.com/go-gl/mathgl v1.0.0
	github.com/google/subcommands v1.2.0
	github.com/miekg/dns v1.1.50
	github.com/sanbornm/go-selfupdate v0.0.0-20210106163404-c9b625feac49
	github.com/sandertv/gophertunnel v1.26.0
	golang.design/x/lockfree v0.0.1
	golang.org/x/exp v0.0.0-20220930202632-ec3f01382ef9
	golang.org/x/oauth2 v0.0.0-20220909003341-f21342109be1
)

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/changkun/lockfree v0.0.1 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.3.0
	github.com/jinzhu/copier v0.3.5
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/kr/binarydist v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/sandertv/go-raknet v1.12.0 // indirect
	github.com/sirupsen/logrus v1.9.0
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20220926161630-eccd6366d1be // indirect
	golang.org/x/image v0.0.0-20220902085622-e7cb96979f69
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/net v0.0.0-20220930213112-107f3e3c3b0b // indirect
	golang.org/x/sys v0.0.0-20220928140112-f11e5e49a4ec // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.12 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inconshreveable/go-update.v0 v0.0.0-20150814200126-d8b0b1d421aa // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)
