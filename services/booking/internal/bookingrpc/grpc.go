package bookingrpc

import (
	pb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/services"
)

type BookingRPC interface {

}

type bookingServer struct {
	pb.UnimplementedBookingServiceServer
}

func NewServer(service services.BookingIntr) *bookingServer {
	return &bookingServer{}
}
