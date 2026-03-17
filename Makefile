.PHONY: proto test lint build up down

SERVICES := staff booking analytics client api-gateway

proto:
	protoc --go_out=. --go_opt=module=github.com/RomanKovalev007/barber_crm \
		--go-grpc_out=. --go-grpc_opt=module=github.com/RomanKovalev007/barber_crm \
		-I api/proto \
		api/proto/staff/v1/staff.proto \
		api/proto/booking/v1/booking.proto \
		api/proto/analytics/v1/analytics.proto \
		api/proto/client/v1/client.proto

test:
	@for svc in $(SERVICES); do \
		echo "==> testing $$svc"; \
		cd services/$$svc && go test ./... -race -count=1 && cd ../..; \
	done

lint:
	@for svc in $(SERVICES); do \
		echo "==> linting $$svc"; \
		cd services/$$svc && golangci-lint run ./... && cd ../..; \
	done

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down
