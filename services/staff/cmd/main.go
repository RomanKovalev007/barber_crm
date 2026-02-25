package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	pb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/redis"
	"github.com/RomanKovalev007/barber_crm/pkg/postgres"
	staffgrpc "github.com/RomanKovalev007/barber_crm/services/staff/internal/grpc"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/repository"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/service"
)

func main() {
	log := logger.New("staff")
	cfg, err := config.ParseStaffConfig()
	if err != nil{
		log.Error("failed to connect to read config", "error", err)
		os.Exit(1)	
	}

	ctx := context.Background()

	pool, err := postgres.NewPostgres(ctx, cfg.DbCfg)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, ttl, err := redis.NewRedis(ctx, cfg.RedisCfg)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	repo := repository.New(pool)
	svc := service.New(repo, rdb, ttl, cfg.JWTSecret, log)
	srv := staffgrpc.NewServer(svc)

	grpcServer := grpc.NewServer()
	pb.RegisterStaffServiceServer(grpcServer, srv)
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("staff service started", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down staff service")
	grpcServer.GracefulStop()
}
