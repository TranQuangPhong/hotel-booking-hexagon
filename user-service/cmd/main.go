package main

import (
	"booking/user-service/internal/adapter/handler"
	"booking/user-service/internal/adapter/postgres"
	"booking/user-service/internal/user"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Create root context listening to OS shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Init repository, service, handler, router, and server
	pool, err := pgxpool.New(ctx, "postgres://userservice:userservice@localhost:5440/db_users") //TODO: use config
	if err != nil {
		fmt.Printf("failed to init postgresql: %s", err.Error())
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		fmt.Printf("failed to ping postgresql server: %s", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	// Init repository, service, handler, router
	userRepository := postgres.NewUserRepository(ctx, pool)
	userService := user.NewUserService(userRepository)
	userHandler := handler.NewUserHandler(userService)
	router := userHandler.UserRouter()

	// Start http server
	server := &http.Server{
		Addr:    ":8181",
		Handler: router,
	}
	go func() {
		//Logging TODO
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			//TODO: log error
			fmt.Printf("failed to start http server: %s", err.Error())
			os.Exit(1)
		}
	}()
	// Logging TODO: server started
	fmt.Println("Server is running...")

	// Block main thread waiting for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Logging TODO: server shutting down
	fmt.Println("Server is shutting down...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		//Logging error TODO
		fmt.Printf("HTTP server forced to shutdown: %s", err.Error())
	} else {
		fmt.Println("HTTP server exiting gracefully")
	}

	// Logging TODO
	fmt.Println("Service exiting gracefully")
}
