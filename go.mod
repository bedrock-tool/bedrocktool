module github.com/bedrock-tool/bedrocktool/bedrock-skin-bot

go 1.19

//replace github.com/sandertv/gophertunnel => ./gophertunnel

replace github.com/sandertv/gophertunnel => github.com/olebeck/gophertunnel v1.24.8-7

require (
	github.com/BurntSushi/toml v1.2.0
	github.com/disgoorg/dislog v1.0.6
	github.com/disgoorg/snowflake v1.1.0
	github.com/fatih/color v1.13.0
	github.com/flytam/filenamify v1.1.1
	github.com/sandertv/gophertunnel v1.24.7
	golang.org/x/exp v0.0.0-20220921164117-439092de6870
	golang.org/x/oauth2 v0.0.0-20220909003341-f21342109be1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/df-mc/atomic v1.10.0 // indirect
	github.com/disgoorg/disgo v0.13.20 // indirect
	github.com/disgoorg/log v1.2.0 // indirect
	github.com/disgoorg/snowflake/v2 v2.0.0 // indirect
	github.com/go-gl/mathgl v1.0.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.3.0
	github.com/klauspost/compress v1.15.10 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/muhammadmuzzammil1998/jsonc v1.0.0 // indirect
	github.com/prometheus/client_golang v1.13.0
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/rabbitmq/amqp091-go v1.5.0
	github.com/sandertv/go-raknet v1.12.0 // indirect
	github.com/sasha-s/go-csync v0.0.0-20210812194225-61421b77c44b // indirect
	github.com/sirupsen/logrus v1.9.0
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20220919173607-35f4265a4bc0 // indirect
	golang.org/x/image v0.0.0-20220902085622-e7cb96979f69 // indirect
	golang.org/x/net v0.0.0-20220921203646-d300de134e69 // indirect
	golang.org/x/sys v0.0.0-20220919091848-fb04ddd9f9c8 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
