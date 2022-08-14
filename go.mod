module bedrocktool

go 1.19

require (
	github.com/df-mc/dragonfly v0.8.1
	github.com/df-mc/goleveldb v1.1.9
	github.com/go-gl/mathgl v1.0.0
	github.com/google/gopacket v1.1.19
	github.com/sandertv/gophertunnel v1.24.0
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e
	golang.org/x/image v0.0.0-20220722155232-062f8c9fd539
	golang.org/x/oauth2 v0.0.0-20220808172628-8227340efae7
)

//replace github.com/sandertv/gophertunnel => ./gophertunnel

//replace github.com/df-mc/dragonfly => ./dragonfly

replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.24.4

replace github.com/df-mc/dragonfly => github.com/olebeck/dragonfly v0.8.2-1

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/sandertv/go-raknet v1.11.1 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/net v0.0.0-20220812174116-3211cb980234 // indirect
	golang.org/x/sys v0.0.0-20220811171246-fbc7d0a398ab // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)
