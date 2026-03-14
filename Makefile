.PHONY: proto

proto:
	protoc --go_out=. --go_opt=module=github.com/RomanKovalev007/barber_crm \
		--go-grpc_out=. --go-grpc_opt=module=github.com/RomanKovalev007/barber_crm \
		-I api/proto \
		api/proto/staff/v1/staff.proto \
		api/proto/booking/v1/booking.proto \
		api/proto/analytics/v1/analytics.proto \
		api/proto/client/v1/client.proto

# Future:
# 		api/proto/notification/v1/notification.proto