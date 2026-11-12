package main

import (
	"booking/booking-service/config"
	bookinggrpc "booking/booking-service/internal/adapter/grpc"
	"booking/booking-service/internal/adapter/handler"
	"booking/booking-service/internal/adapter/postgres"
	"booking/booking-service/internal/booking"
	bookingv1 "booking/contracts/booking/v1"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	logger "github.com/TranQuangPhong/hotel-booking-logger"
	grpclogger "github.com/TranQuangPhong/hotel-booking-logger/grpclogger"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

func main() {
	// This process lives in UTC: pgx scans TIMESTAMPTZ in time.Local, and the contracts promise RFC 3339 UTC
	time.Local = time.UTC

	// 1. Initialize structured logger FIRST
	log := logger.NewLogger()
	slog.SetDefault(log)

	// 2. Create a root context that listens for OS shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load env config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Init postgresql
	connString := fmt.Sprintf( // Eg: "postgres://booking:booking@localhost:5440/booking_service"
		"postgres://%s:%s@%s:%d/%s",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		slog.Error("failed to init postgresql", "error", err)
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping postgresql server", "error", err)
		pool.Close()
		os.Exit(1)
	}
	defer pool.Close()

	// Init repository, service, handler, router
	bookingRepository := postgres.NewBookingRepository(pool)
	bookingService := booking.NewService(bookingRepository)
	bookingHandler := handler.New(bookingService)

	router := bookingHandler.BookingRouter()

	// Start http server
	httpServerPort := fmt.Sprintf(":%d", cfg.Server.Port) //Eg: ":8080"
	server := &http.Server{
		Addr:    httpServerPort,
		Handler: router,
	}
	go func() {
		slog.Info("Server is starting...", "addr", httpServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start http server", "error", err)
			os.Exit(1)
		}
	}()

	// Start grpc server
	grpcPort := fmt.Sprintf(":%d", cfg.GRPC.Port)
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		slog.Error("grpc: failed to listen", "addr", grpcPort, "error", err)
		os.Exit(1)
	}
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			// first: it must wrap recovery, so a recovered panic is still logged as Internal
			grpclogger.UnaryServerInterceptor(),
			// Without a handler, v2 sends a panic to the client as UNKNOWN + stack trace and logs nothing
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(
				func(ctx context.Context, p any) error {
					slog.ErrorContext(ctx, "grpc handler panic", "panic", p, "stack", string(debug.Stack()))
					return status.Error(codes.Internal, "internal server error")
				},
			)),
		),
	}
	grpcServer := grpc.NewServer(opts...)
	bookingv1.RegisterBookingServiceServer(grpcServer, bookinggrpc.New(bookingService))
	reflection.Register(grpcServer) // dev: grpcurl list/describe without -proto; gate it before deploying
	go func() {
		slog.Info("grpc server is starting...", "addr", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("failed to start grpc server", "error", err)
			os.Exit(1)
		}
	}()

	// Block main thread waiting for shutdown signal
	<-ctx.Done()
	slog.Info("Shutdown signal received, exiting")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shutdown", "error", err)
	} else {
		slog.Info("HTTP server exiting gracefully")
	}

	// GracefulStop has no time limit: Stop force-closes the running RPCs once shutdownCtx expires
	cancelForceStop := context.AfterFunc(shutdownCtx, grpcServer.Stop)
	grpcServer.GracefulStop()
	if cancelForceStop() {
		slog.Info("gRPC server exiting gracefully")
	} else {
		slog.Error("gRPC server forced to stop")
	}
	slog.Info("Service exiting gracefully")
}
