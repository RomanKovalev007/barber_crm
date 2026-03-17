package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/pkg/postgres"
	"github.com/RomanKovalev007/barber_crm/pkg/redis"
	staffgrpc "github.com/RomanKovalev007/barber_crm/services/staff/internal/grpc"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/kafka"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/repository"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/service"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func main() {
	log := logger.New("staff")
	cfg, err := config.ParseStaffConfig()
	if err != nil {
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

	producer := kafka.NewProducer(cfg.KafkaCfg.Brokers)
	defer producer.Close()

	if err := producer.Ping(ctx); err != nil {
		log.Error("failed to connect to kafka", "error", err)
		os.Exit(1)
	}

	repo := repository.New(pool)
	svc := service.New(repo, repository.NewRedisStore(rdb), producer, ttl, cfg.JWTSecret, log)
	srv := staffgrpc.NewServer(svc)

	recoveryFn := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}

	grpc_prometheus.EnableHandlingTimeHistogram()

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpc_prometheus.UnaryServerInterceptor, recoveryFn),
	)
	pb.RegisterStaffServiceServer(grpcServer, srv)
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	grpc_prometheus.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = ":9091"
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Info("metrics server started", "port", metricsPort)
		if err := http.ListenAndServe(metricsPort, mux); err != nil {
			log.Error("metrics server failed", "error", err)
		}
	}()

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

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	select {
	case <-stopped:
		log.Info("graceful shutdown complete")
	case <-shutdownCtx.Done():
		log.Warn("graceful shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
}
