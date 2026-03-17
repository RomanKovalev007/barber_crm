package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/client/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	"github.com/RomanKovalev007/barber_crm/pkg/postgres"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/consumer"
	clientgrpc "github.com/RomanKovalev007/barber_crm/services/client/internal/grpc"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/repository"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/service"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log := logger.New("client")

	cfg, err := config.ParseClientConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── PostgreSQL ───────────────────────────────────────────────────────────
	pool, err := postgres.NewPostgres(ctx, cfg.DbCfg)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ── Repository & Service ─────────────────────────────────────────────────
	repo := repository.New(pool)
	svc := service.New(repo, log)
	srv := clientgrpc.NewServer(svc)

	// ── gRPC ─────────────────────────────────────────────────────────────────
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
	pb.RegisterClientServiceServer(grpcServer, srv)
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
		metricsPort = ":9094"
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
		log.Info("client service started", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server failed", "error", err)
		}
	}()

	// ── Kafka consumer ───────────────────────────────────────────────────────
	brokers := strings.Split(cfg.KafkaCfg.Brokers, ",")
	cons := consumer.New(brokers, repo, log)

	cons.Start(ctx) // блокирует до отмены ctx

	log.Info("shutting down client service")

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()
	select {
	case <-stopped:
		log.Info("graceful shutdown complete")
	case <-shutdownCtx.Done():
		log.Warn("graceful shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
	cons.Close()
}
