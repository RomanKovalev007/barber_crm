package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/config"
	"github.com/RomanKovalev007/barber_crm/pkg/logger"
	analyticsgrpc "github.com/RomanKovalev007/barber_crm/services/analytics/internal/grpc"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/consumer"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/repository"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/service"
)

func main() {
	log := logger.New("analytics")

	cfg, err := config.ParseAnalyticsConfig()
	if err != nil {
		log.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── ClickHouse ──────────────────────────────────────────────────────────
	chConn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.CHCfg.Host + ":" + cfg.CHCfg.Port},
		Auth: clickhouse.Auth{
			Database: cfg.CHCfg.Database,
			Username: cfg.CHCfg.Username,
			Password: cfg.CHCfg.Password,
		},
		DialTimeout:  5 * time.Second,
		MaxOpenConns: 10,
	})
	if err != nil {
		log.Error("failed to open clickhouse", "error", err)
		os.Exit(1)
	}
	if err := chConn.Ping(ctx); err != nil {
		log.Error("failed to ping clickhouse", "error", err)
		os.Exit(1)
	}
	defer chConn.Close()

	// ── Repository ──────────────────────────────────────────────────────────
	repo := repository.New(chConn)

	// ── Service & gRPC ──────────────────────────────────────────────────────
	svc := service.New(repo, log)
	srv := analyticsgrpc.NewServer(svc)

	grpcServer := grpc.NewServer()
	pb.RegisterAnalyticsServiceServer(grpcServer, srv)
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("analytics service started", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server failed", "error", err)
		}
	}()

	// ── Kafka consumer ──────────────────────────────────────────────────────
	brokers := strings.Split(cfg.KafkaCfg.Brokers, ",")
	cons := consumer.New(brokers, repo, log)

	cons.Start(ctx) // блокирует до отмены ctx

	log.Info("shutting down analytics service")
	grpcServer.GracefulStop()
	cons.Close()
}
