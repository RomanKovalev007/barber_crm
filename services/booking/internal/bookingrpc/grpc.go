package bookingrpc

import (
	pb "github.com/RomanKovalev007/barber_crm/api/proto"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/services"
)

type BookingRPC interface {

}

type bookingServer struct {
	pb.UnimplementedBookingServer
}

func NewServer(service services.BookingIntr) BookingRPC {
	return &bookingServer{}
}
