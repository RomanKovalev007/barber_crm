package handler

import (
	analyticsv1 "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	bookingv1 "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	clientv1 "github.com/RomanKovalev007/barber_crm/api/proto/client/v1"
	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/internal/model"
)

// ─── Booking ──────────────────────────────────────────────────────────────────

func bookingToModel(b *bookingv1.Booking) model.Booking {
	return model.Booking{
		ID:          b.BookingId,
		ClientName:  b.ClientName,
		ClientPhone: b.ClientPhone,
		BarberID:    b.BarberId,
		ServiceID:   b.ServiceId,
		ServiceName: b.ServiceName,
		Price:       b.Price,
		TimeStart:   b.TimeStart.AsTime(),
		TimeEnd:     b.TimeEnd.AsTime(),
		Status:      bookingStatusToModel(b.Status),
	}
}

func bookingStatusToModel(s bookingv1.BookingStatus) string {
	switch s {
	case bookingv1.BookingStatus_BOOKING_STATUS_PENDING:
		return model.StatusPending
	case bookingv1.BookingStatus_BOOKING_STATUS_COMPLETED:
		return model.StatusCompleted
	case bookingv1.BookingStatus_BOOKING_STATUS_CANCELLED:
		return model.StatusCancelled
	case bookingv1.BookingStatus_BOOKING_STATUS_NO_SHOW:
		return model.StatusNoShow
	default:
		return model.StatusPending
	}
}

func bookingStatusToProto(s string) bookingv1.BookingStatus {
	switch s {
	case "pending":
		return bookingv1.BookingStatus_BOOKING_STATUS_PENDING
	case "cancelled":
		return bookingv1.BookingStatus_BOOKING_STATUS_CANCELLED
	case "completed":
		return bookingv1.BookingStatus_BOOKING_STATUS_COMPLETED
	case "no_show":
		return bookingv1.BookingStatus_BOOKING_STATUS_NO_SHOW
	default:
		return bookingv1.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
}

// ─── Slot ─────────────────────────────────────────────────────────────────────

func slotsToModel(pbSlots []*bookingv1.Slot) []model.Slot {
	slots := make([]model.Slot, 0, len(pbSlots))
	for _, s := range pbSlots {
		slot := model.Slot{
			Status:    slotStatusToModel(s.Status),
			TimeStart: s.TimeStart.AsTime(),
			TimeEnd:   s.TimeEnd.AsTime(),
		}
		if s.Booking != nil {
			slot.Booking = &model.SlotBooking{
				BookingID:   s.Booking.BookingId,
				ClientName:  s.Booking.ClientName,
				ClientPhone: s.Booking.ClientPhone,
				ServiceName: s.Booking.ServiceName,
				Status:      bookingStatusToModel(s.Booking.Status),
			}
		}
		slots = append(slots, slot)
	}
	return slots
}

func slotStatusToModel(s bookingv1.SlotStatus) model.SlotStatus {
	switch s {
	case bookingv1.SlotStatus_SLOT_STATUS_FREE:
		return model.SlotFree
	case bookingv1.SlotStatus_SLOT_STATUS_BOOKED:
		return model.SlotBooked
	case bookingv1.SlotStatus_SLOT_STATUS_BLOCKED:
		return model.SlotBlocked
	default:
		return model.SlotFree
	}
}

// ─── Staff ────────────────────────────────────────────────────────────────────

func barberToModel(b *staffv1.BarberResponse) model.Barber {
	services := make([]model.Service, 0, len(b.Services))
	for _, s := range b.Services {
		services = append(services, serviceToModel(s))
	}
	return model.Barber{
		ID:       b.BarberId,
		Name:     b.Name,
		Services: services,
	}
}

func serviceToModel(s *staffv1.ServiceResponse) model.Service {
	return model.Service{
		ID:              s.ServiceId,
		Name:            s.Name,
		Price:           s.Price,
		DurationMinutes: s.DurationMinutes,
		IsActive:        s.IsActive,
	}
}

func scheduleDayToModel(d *staffv1.ScheduleDay) model.ScheduleDay {
	return model.ScheduleDay{
		ID:        d.ScheduleDayId,
		BarberID:  d.BarberId,
		Date:      d.Date,
		StartTime: d.StartTime,
		EndTime:   d.EndTime,
		PartOfDay: partOfDayToModel(d.PartOfDay),
	}
}

func partOfDayToModel(p staffv1.PartOfDay) model.PartOfDay {
	switch p {
	case staffv1.PartOfDay_PART_OF_DAY_AM:
		return model.PartOfDayAM
	case staffv1.PartOfDay_PART_OF_DAY_PM:
		return model.PartOfDayPM
	default:
		return model.PartOfDayAM
	}
}

func partOfDayToProto(p string) staffv1.PartOfDay {
	switch p {
	case "am":
		return staffv1.PartOfDay_PART_OF_DAY_AM
	case "pm":
		return staffv1.PartOfDay_PART_OF_DAY_PM
	default:
		return staffv1.PartOfDay_PART_OF_DAY_UNSPECIFIED
	}
}

// ─── Client ───────────────────────────────────────────────────────────────────

func clientToModel(c *clientv1.Client) model.Client {
	client := model.Client{
		ID:          c.ClientId,
		Name:        c.Name,
		Phone:       c.Phone,
		Notes:       c.Notes,
		VisitsCount: c.VisitsCount,
	}
	if c.LastVisit != nil && c.LastVisit.IsValid() {
		client.LastVisit = c.LastVisit.AsTime().Format("2006-01-02")
	}
	return client
}

// ─── Analytics ────────────────────────────────────────────────────────────────

func analyticsToModel(r *analyticsv1.BarberStatsResponse) model.BarberStats {
	topServices := make([]model.TopService, 0, len(r.TopServices))
	for _, s := range r.TopServices {
		topServices = append(topServices, model.TopService{
			ServiceID:   s.ServiceId,
			ServiceName: s.ServiceName,
			Count:       s.Count,
			Revenue:     s.Revenue,
		})
	}

	daily := make([]model.DayStat, 0, len(r.DailyBreakdown))
	for _, d := range r.DailyBreakdown {
		daily = append(daily, model.DayStat{
			Date:        d.Date,
			Clients:     d.Clients,
			Revenue:     d.Revenue,
			HoursWorked: d.HoursWorked,
		})
	}

	return model.BarberStats{
		BarberID:          r.BarberId,
		DateFrom:          r.DateFrom,
		DateTo:            r.DateTo,
		ClientsServed:     r.ClientsServed,
		TotalRevenue:      r.TotalRevenue,
		HoursWorked:       r.HoursWorked,
		AverageCheck:      r.AverageCheck,
		BookingsTotal:     r.BookingsTotal,
		BookingsCompleted: r.BookingsCompleted,
		BookingsCancelled: r.BookingsCancelled,
		BookingsNoShow:    r.BookingsNoShow,
		OccupancyRate:     r.OccupancyRate,
		TopServices:       topServices,
		DailyBreakdown:    daily,
	}
}
