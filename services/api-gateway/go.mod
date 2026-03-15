module github.com/RomanKovalev007/barber_crm/services/api-gateway

go 1.24.3

require (
	github.com/RomanKovalev007/barber_crm/api v0.0.8
	github.com/RomanKovalev007/barber_crm/pkg v0.0.6
	github.com/go-chi/chi/v5 v5.2.5
	github.com/google/uuid v1.6.0
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/ilyakaznacheev/cleanenv v1.5.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)

replace (
	github.com/RomanKovalev007/barber_crm/api => ../../api
	github.com/RomanKovalev007/barber_crm/pkg => ../../pkg
)
