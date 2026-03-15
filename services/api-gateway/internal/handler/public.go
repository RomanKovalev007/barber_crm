package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	bookingv1 "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/pkg/response"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PublicHandler struct {
	staff   staffv1.StaffServiceClient
	booking bookingv1.BookingServiceClient
}

func NewPublicHandler(staff staffv1.StaffServiceClient, booking bookingv1.BookingServiceClient) *PublicHandler {
	return &PublicHandler{staff: staff, booking: booking}
}

func (h *PublicHandler) ListBarbers(w http.ResponseWriter, r *http.Request) {
	resp, err := h.staff.ListBarbers(r.Context(), &staffv1.ListBarbersRequest{})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	barbers := make([]model.Barber, 0, len(resp.Barbers))
	for _, b := range resp.Barbers {
		barbers = append(barbers, barberToModel(b))
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"barbers": barbers})
}

func (h *PublicHandler) ListServicesByBarber(w http.ResponseWriter, r *http.Request) {
	barberID := chi.URLParam(r, "barber_id")
	if err := validateBarberID(barberID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	resp, err := h.staff.ListServices(r.Context(), &staffv1.ListServicesRequest{
		BarberId:        barberID,
		IncludeInactive: false,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	services := make([]model.Service, 0, len(resp.Services))
	for _, s := range resp.Services {
		services = append(services, serviceToModel(s))
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"services": services})
}

func (h *PublicHandler) FreeSlotsByBarber(w http.ResponseWriter, r *http.Request) {
	barberID := chi.URLParam(r, "barber_id")
	if err := validateBarberID(barberID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// date is optional — if empty, booking service returns the nearest available day
	date := r.URL.Query().Get("date")
	if date != "" {
		if err := validateDate(date); err != nil {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid date format, use YYYY-MM-DD")
			return
		}
	}

	// service_id is optional
	serviceID := r.URL.Query().Get("service_id")
	if serviceID != "" {
		if err := validateServiceID(serviceID); err != nil {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid service_id")
			return
		}
	}

	resp, err := h.booking.GetFreeSlots(r.Context(), &bookingv1.FreeSlotsRequest{
		BarberId:  barberID,
		Date:      date,
		ServiceId: serviceID,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"barber_id": resp.BarberId,
		"date":      resp.Date,
		"slots":     slotsToModel(resp.Slots),
	})
}

func (h *PublicHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req model.Booking

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if err := validateBookingRequest(req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	resp, err := h.booking.CreateBooking(r.Context(), &bookingv1.CreateBookingRequest{
		BarberId:    req.BarberID,
		ServiceId:   req.ServiceID,
		ClientName:  req.ClientName,
		ClientPhone: req.ClientPhone,
		TimeStart:   timestamppb.New(req.TimeStart),
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	b := bookingToModel(resp.Booking)
	response.WriteJSON(w, http.StatusCreated, b)
}
