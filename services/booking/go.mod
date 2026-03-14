module github.com/RomanKovalev007/barber_crm/services/booking

go 1.24.3

require (
	github.com/RomanKovalev007/barber_crm/api v0.0.7
	github.com/RomanKovalev007/barber_crm/pkg v0.0.3
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.8.0
	github.com/redis/go-redis/v9 v9.18.0
	github.com/segmentio/kafka-go v0.4.47
	github.com/stretchr/testify v1.11.1
	google.golang.org/grpc v1.79.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/ilyakaznacheev/cleanenv v1.5.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)

replace (
	github.com/RomanKovalev007/barber_crm/api => ../../api
	github.com/RomanKovalev007/barber_crm/pkg => ../../pkg
)
