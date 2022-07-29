module bedrocktool

go 1.17

require (
	github.com/df-mc/dragonfly v0.7.5
	github.com/df-mc/goleveldb v1.1.9
	github.com/google/gopacket v1.1.19
	github.com/sandertv/gophertunnel v1.22.3
	golang.org/x/oauth2 v0.0.0-20220722155238-128564f6959c
)

//replace github.com/sandertv/gophertunnel => ./gophertunnel
//replace github.com/df-mc/dragonfly => ./dragonfly

replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.22.4

require (
	github.com/brentp/intintmap v0.0.0-20190211203843-30dc0ade9af9 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/go-gl/mathgl v1.0.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/sandertv/go-raknet v1.11.1 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/image v0.0.0-20220722155232-062f8c9fd539 // indirect
	golang.org/x/net v0.0.0-20220728211354-c7608f3a8462 // indirect
	golang.org/x/sys v0.0.0-20220728004956-3c1f35247d10 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)
