package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/pkg/postgres"
	"github.com/RomanKovalev007/barber_crm/pkg/redis"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/bookingrpc"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/services"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/staffclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	log := logger.New("booking")
	cfg, err := config.ParseBookingConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := postgres.NewPostgres(ctx, cfg.DbCfg)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient, ttl, err := redis.NewRedis(ctx, cfg.RedisCfg)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	staffClient, err := staffclient.New(cfg.StaffGRPCAddr)
	if err != nil {
		log.Error("failed to connect to staff service", "error", err)
		os.Exit(1)
	}
	defer staffClient.Close()

	bookingRepo := repo.New(pool)
	bookingService := services.New(bookingRepo, redisClient, ttl, cfg.JWTSecret, log, staffClient)
	srv := bookingrpc.NewServer(bookingService)

	grpcServer := grpc.NewServer()
	pb.RegisterBookingServiceServer(grpcServer, srv)
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("booking service started", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server failed", "error", err)
		}
	}()

	quiet := make(chan os.Signal, 1)
	signal.Notify(quiet, syscall.SIGINT, syscall.SIGTERM)
	<-quiet

	log.Info("shutting down booking service")
	grpcServer.GracefulStop()
}
