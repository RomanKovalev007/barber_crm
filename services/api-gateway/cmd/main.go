package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	analyticsv1 "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	bookingv1 "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	clientv1 "github.com/RomanKovalev007/barber_crm/api/proto/client/v1"
	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/internal/handler"
	"github.com/RomanKovalev007/barber_crm/services/api-gateway/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log := logger.New("api-gateway")

	cfg, err := config.ParseApiGatewayConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	// ── gRPC connections ──────────────────────────────────────────────────────

	staffConn, err := grpc.NewClient(cfg.StaffAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("invalid staff service address", "addr", cfg.StaffAddr, "error", err)
		os.Exit(1)
	}
	defer staffConn.Close()

	bookingConn, err := grpc.NewClient(cfg.BookingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("invalid booking service address", "addr", cfg.BookingAddr, "error", err)
		os.Exit(1)
	}
	defer bookingConn.Close()

	analyticsConn, err := grpc.NewClient(cfg.AnalyticsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("invalid analytics service address", "addr", cfg.AnalyticsAddr, "error", err)
		os.Exit(1)
	}
	defer analyticsConn.Close()

	clientConn, err := grpc.NewClient(cfg.ClientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("invalid client service address", "addr", cfg.ClientAddr, "error", err)
		os.Exit(1)
	}
	defer clientConn.Close()

	// ── gRPC clients ──────────────────────────────────────────────────────────

	staffClient := staffv1.NewStaffServiceClient(staffConn)
	bookingClient := bookingv1.NewBookingServiceClient(bookingConn)
	analyticsClient := analyticsv1.NewAnalyticsServiceClient(analyticsConn)
	clientClient := clientv1.NewClientServiceClient(clientConn)

	// ── Handlers ──────────────────────────────────────────────────────────────

	publicHandler := handler.NewPublicHandler(staffClient, bookingClient)
	authHandler := handler.NewAuthHandler(staffClient)
	staffHandler := handler.NewStaffHandler(staffClient, bookingClient, clientClient, analyticsClient)

	// ── Router ────────────────────────────────────────────────────────────────

	rateLimiter := middleware.NewRateLimiter(10, 30)

	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.Logger(log))
	r.Use(middleware.BodyLimit)
	r.Use(rateLimiter.Middleware)
	r.Use(middleware.Metrics)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Metrics — без аутентификации и таймаута
	r.Handle("/metrics", promhttp.Handler())

	// Health check — без таймаута и аутентификации
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Public — 5s достаточно для простых запросов
		r.Group(func(r chi.Router) {
			r.Use(chiMiddleware.Timeout(5 * time.Second))
			r.Get("/barbers", publicHandler.ListBarbers)
			r.Get("/barbers/{barber_id}/services", publicHandler.ListServicesByBarber)
			r.Get("/barbers/{barber_id}/free-slots", publicHandler.FreeSlotsByBarber)
			r.Post("/bookings", publicHandler.CreateBooking)
		})

		// Auth — 5s
		r.Group(func(r chi.Router) {
			r.Use(chiMiddleware.Timeout(5 * time.Second))
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/refresh", authHandler.Refresh)
			r.With(middleware.Auth(cfg.JWTSecret)).Post("/auth/logout", authHandler.Logout)
		})

		// Staff (все требуют JWT)
		r.Route("/staff", func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))

			// Стандартные операции — 10s
			r.Group(func(r chi.Router) {
				r.Use(chiMiddleware.Timeout(10 * time.Second))

				r.Get("/barber", staffHandler.GetBarber)

				r.Get("/services", staffHandler.ListServices)
				r.Post("/services", staffHandler.CreateService)
				r.Put("/services/{service_id}", staffHandler.UpdateService)
				r.Delete("/services/{service_id}", staffHandler.DeleteService)

				r.Get("/schedule", staffHandler.GetSchedule)
				r.Put("/schedule/{date}", staffHandler.UpsertSchedule)
				r.Delete("/schedule/{date}", staffHandler.DeleteSchedule)
				r.Get("/slots", staffHandler.GetSlots)

				r.Post("/bookings", staffHandler.CreateBooking)
				r.Get("/bookings/{booking_id}", staffHandler.GetBooking)
				r.Put("/bookings/{booking_id}", staffHandler.UpdateBooking)
				r.Patch("/bookings/{booking_id}", staffHandler.UpdateBookingStatus)
				r.Delete("/bookings/{booking_id}", staffHandler.DeleteBooking)

				r.Get("/clients", staffHandler.ListClients)
				r.Get("/clients/{client_id}", staffHandler.GetClient)
				r.Put("/clients/{client_id}", staffHandler.UpdateClient)
			})

			// Аналитика — 30s (тяжёлые запросы к ClickHouse)
			r.With(chiMiddleware.Timeout(30 * time.Second)).Get("/analytics", staffHandler.GetAnalytics)
		})
	})

	// ── HTTP server ───────────────────────────────────────────────────────────

	srv := &http.Server{
		Addr:         cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("api-gateway started", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down api-gateway")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	} else {
		log.Info("graceful shutdown complete")
	}
}
