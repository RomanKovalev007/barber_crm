package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	analyticsv1 "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	bookingv1 "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	clientv1 "github.com/RomanKovalev007/barber_crm/api/proto/client/v1"
	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/internal/middleware"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/pkg/response"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StaffHandler struct {
	staff     staffv1.StaffServiceClient
	booking   bookingv1.BookingServiceClient
	client    clientv1.ClientServiceClient
	analytics analyticsv1.AnalyticsServiceClient
}

func NewStaffHandler(
	staff staffv1.StaffServiceClient,
	booking bookingv1.BookingServiceClient,
	client clientv1.ClientServiceClient,
	analytics analyticsv1.AnalyticsServiceClient,
) *StaffHandler {
	return &StaffHandler{
		staff:     staff,
		booking:   booking,
		client:    client,
		analytics: analytics,
	}
}

// ─── Barber ───────────────────────────────────────────────────────────────────

func (h *StaffHandler) GetBarber(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())

	resp, err := h.staff.GetBarber(r.Context(), &staffv1.GetBarberRequest{BarberId: barberID})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, barberToModel(resp))
}

// ─── Services ─────────────────────────────────────────────────────────────────

func (h *StaffHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())

	// default true (include inactive), unless explicitly set to false
	includeInactive := r.URL.Query().Get("include_inactive") != "false"

	resp, err := h.staff.ListServices(r.Context(), &staffv1.ListServicesRequest{
		BarberId:        barberID,
		IncludeInactive: includeInactive,
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

func (h *StaffHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())

	var req struct {
		Name            string `json:"name"`
		Price           int32  `json:"price"`
		DurationMinutes int32  `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.Name == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "name is required")
		return
	}
	if req.Price < 0 {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "price cannot be negative")
		return
	}
	if req.DurationMinutes <= 0 || req.DurationMinutes%15 != 0 {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "duration_minutes must be a positive multiple of 15")
		return
	}

	resp, err := h.staff.CreateService(r.Context(), &staffv1.CreateServiceRequest{
		BarberId:        barberID,
		Name:            req.Name,
		Price:           req.Price,
		DurationMinutes: req.DurationMinutes,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, serviceToModel(resp))
}

func (h *StaffHandler) UpdateService(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	serviceID := chi.URLParam(r, "service_id")
	if err := validateServiceID(serviceID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid service_id")
		return
	}

	var req struct {
		Name            string `json:"name"`
		Price           int32  `json:"price"`
		DurationMinutes int32  `json:"duration_minutes"`
		IsActive        bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.Name == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "name is required")
		return
	}
	if req.Price < 0 {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "price cannot be negative")
		return
	}
	if req.DurationMinutes <= 0 || req.DurationMinutes%15 != 0 {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "duration_minutes must be a positive multiple of 15")
		return
	}

	resp, err := h.staff.UpdateService(r.Context(), &staffv1.UpdateServiceRequest{
		ServiceId:       serviceID,
		BarberId:        barberID,
		Name:            req.Name,
		Price:           req.Price,
		DurationMinutes: req.DurationMinutes,
		IsActive:        req.IsActive,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, serviceToModel(resp))
}

func (h *StaffHandler) DeleteService(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	serviceID := chi.URLParam(r, "service_id")
	if err := validateServiceID(serviceID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid service_id")
		return
	}

	_, err := h.staff.DeleteService(r.Context(), &staffv1.DeleteServiceRequest{
		ServiceId: serviceID,
		BarberId:  barberID,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Schedule ─────────────────────────────────────────────────────────────────

func (h *StaffHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	week := r.URL.Query().Get("week")
	if !isValidWeek(week) {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "week must be in format YYYY-Www (e.g. 2026-W10)")
		return
	}

	resp, err := h.staff.GetSchedule(r.Context(), &staffv1.GetScheduleRequest{
		BarberId: barberID,
		Week:     week,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	days := make([]model.ScheduleDay, 0, len(resp.Days))
	for _, d := range resp.Days {
		days = append(days, scheduleDayToModel(d))
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"week": resp.Week, "days": days})
}

func (h *StaffHandler) UpsertSchedule(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	date := chi.URLParam(r, "date")
	if err := validateDate(date); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid date format, use YYYY-MM-DD")
		return
	}

	var req struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		PartOfDay string `json:"part_of_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.StartTime == "" || req.EndTime == "" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "start_time and end_time are required")
		return
	}
	if req.PartOfDay != "am" && req.PartOfDay != "pm" {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "part_of_day must be 'am' or 'pm'")
		return
	}

	resp, err := h.staff.UpsertSchedule(r.Context(), &staffv1.UpsertScheduleRequest{
		BarberId:  barberID,
		Date:      date,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		PartOfDay: partOfDayToProto(req.PartOfDay),
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, scheduleDayToModel(resp))
}

func (h *StaffHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	date := chi.URLParam(r, "date")
	if err := validateDate(date); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid date format, use YYYY-MM-DD")
		return
	}

	_, err := h.staff.DeleteSchedule(r.Context(), &staffv1.DeleteScheduleRequest{
		BarberId: barberID,
		Date:     date,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *StaffHandler) GetSlots(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	date := r.URL.Query().Get("date")
	if err := validateDate(date); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "date is required in format YYYY-MM-DD")
		return
	}

	resp, err := h.booking.GetSlots(r.Context(), &bookingv1.SlotsRequest{
		BarberId: barberID,
		Date:     date,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"date":  resp.Date,
		"slots": slotsToModel(resp.Slots),
	})
}

// ─── Bookings ─────────────────────────────────────────────────────────────────

func (h *StaffHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())

	var req struct {
		ServiceID   string    `json:"service_id"`
		ClientName  string    `json:"client_name"`
		ClientPhone string    `json:"client_phone"`
		TimeStart   time.Time `json:"time_start"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := validateServiceID(req.ServiceID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid service_id")
		return
	}
	if err := validateClientName(req.ClientName); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "client_name is required")
		return
	}
	if err := validateClientPhone(req.ClientPhone); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid client_phone")
		return
	}
	if err := validateTime(req.TimeStart); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid time_start")
		return
	}

	resp, err := h.booking.CreateBooking(r.Context(), &bookingv1.CreateBookingRequest{
		BarberId:    barberID,
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

func (h *StaffHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "booking_id")
	if err := validateBookingID(bookingID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid booking_id")
		return
	}

	resp, err := h.booking.GetBooking(r.Context(), &bookingv1.BookingIdRequest{BookingId: bookingID})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, bookingToModel(resp.Booking))
}

func (h *StaffHandler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	bookingID := chi.URLParam(r, "booking_id")
	if err := validateBookingID(bookingID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid booking_id")
		return
	}

	var req struct {
		ServiceID string    `json:"service_id"`
		TimeStart time.Time `json:"time_start"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	grpcReq := &bookingv1.UpdateBookingRequest{
		BookingId: bookingID,
		BarberId:  barberID,
	}
	if req.ServiceID != "" {
		if err := validateServiceID(req.ServiceID); err != nil {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid service_id")
			return
		}
		grpcReq.ServiceId = req.ServiceID
	}
	if !req.TimeStart.IsZero() {
		grpcReq.TimeStart = timestamppb.New(req.TimeStart)
	}

	resp, err := h.booking.UpdateBooking(r.Context(), grpcReq)
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, bookingToModel(resp.Booking))
}

func (h *StaffHandler) UpdateBookingStatus(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	bookingID := chi.URLParam(r, "booking_id")
	if err := validateBookingID(bookingID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid booking_id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if !allowedBookingStatuses[req.Status] {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "status must be one of: cancelled, completed, no_show")
		return
	}

	resp, err := h.booking.UpdateBookingStatus(r.Context(), &bookingv1.UpdateBookingStatusRequest{
		BookingId: bookingID,
		BarberId:  barberID,
		Status:    bookingStatusToProto(req.Status),
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, bookingToModel(resp.Booking))
}

func (h *StaffHandler) DeleteBooking(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "booking_id")
	if err := validateBookingID(bookingID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid booking_id")
		return
	}

	_, err := h.booking.DeleteBooking(r.Context(), &bookingv1.BookingIdRequest{BookingId: bookingID})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Clients ──────────────────────────────────────────────────────────────────

func (h *StaffHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())
	search := r.URL.Query().Get("search")

	resp, err := h.client.ListClients(r.Context(), &clientv1.ListClientsRequest{
		BarberId: barberID,
		Search:   search,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	clients := make([]model.Client, 0, len(resp.Clients))
	for _, c := range resp.Clients {
		clients = append(clients, clientToModel(c))
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"clients": clients})
}

func (h *StaffHandler) GetClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "client_id")
	if err := validateClientID(clientID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid client_id")
		return
	}

	resp, err := h.client.GetClient(r.Context(), &clientv1.GetClientRequest{ClientId: clientID})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, clientToModel(resp.Client))
}

func (h *StaffHandler) UpdateClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "client_id")
	if err := validateClientID(clientID); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid client_id")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	resp, err := h.client.UpdateClient(r.Context(), &clientv1.UpdateClientRequest{
		ClientId: clientID,
		Name:     req.Name,
		Notes:    req.Notes,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, clientToModel(resp.Client))
}

// ─── Analytics ────────────────────────────────────────────────────────────────

func (h *StaffHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	barberID := middleware.BarberIDFromCtx(r.Context())

	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")
	period := r.URL.Query().Get("period")

	var analyticsperiod *analyticsv1.Period

	if dateFrom != "" && dateTo != "" {
		// Custom range takes priority
		if err := validateDate(dateFrom); err != nil {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid date_from format, use YYYY-MM-DD")
			return
		}
		if err := validateDate(dateTo); err != nil {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "invalid date_to format, use YYYY-MM-DD")
			return
		}
		analyticsperiod = &analyticsv1.Period{
			Kind: &analyticsv1.Period_Custom{
				Custom: &analyticsv1.DateRange{
					DateFrom: dateFrom,
					DateTo:   dateTo,
				},
			},
		}
	} else if period != "" {
		preset, ok := periodPresets[period]
		if !ok {
			response.ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "period must be one of: day, week, month, all")
			return
		}
		analyticsperiod = &analyticsv1.Period{
			Kind: &analyticsv1.Period_Preset{Preset: preset},
		}
	} else {
		// Default: all time
		analyticsperiod = &analyticsv1.Period{
			Kind: &analyticsv1.Period_Preset{Preset: analyticsv1.PredefinedPeriod_PERIOD_ALL},
		}
	}

	resp, err := h.analytics.GetBarberStats(r.Context(), &analyticsv1.GetBarberStatsRequest{
		BarberId: barberID,
		Period:   analyticsperiod,
	})
	if err != nil {
		response.GrpcErrorToHttp(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, analyticsToModel(resp))
}

var periodPresets = map[string]analyticsv1.PredefinedPeriod{
	"day":   analyticsv1.PredefinedPeriod_PERIOD_DAY,
	"week":  analyticsv1.PredefinedPeriod_PERIOD_WEEK,
	"month": analyticsv1.PredefinedPeriod_PERIOD_MONTH,
	"all":   analyticsv1.PredefinedPeriod_PERIOD_ALL,
}

var allowedBookingStatuses = map[string]bool{
	"cancelled": true,
	"completed": true,
	"no_show":   true,
}
