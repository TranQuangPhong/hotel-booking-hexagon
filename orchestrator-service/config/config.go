package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server  Server // REST
	Clients Clients
	DB      DB
	Kafka   Kafka
	Saga    Saga
}

type Server struct {
	Port int
}

type Clients struct { // gRPC clients of the participants
	RoomAddr    string        // ROOM_GRPC_ADDR, e.g. localhost:9180
	BookingAddr string        // BOOKING_GRPC_ADDR
	Timeout     time.Duration // GRPC_TIMEOUT, 3s: each call gets its own context.WithTimeout
}

type DB struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type Kafka struct {
	Brokers []string
}

type Saga struct {
	HoldTTL time.Duration // HOLD_TTL, 15m
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil {
		// Ignore because production may not have .env.
	}

	serverPort, err := strconv.Atoi(os.Getenv("SERVER_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}

	roomAddr, err := required("ROOM_GRPC_ADDR")
	if err != nil {
		return Config{}, err
	}
	bookingAddr, err := required("BOOKING_GRPC_ADDR")
	if err != nil {
		return Config{}, err
	}
	clientTimeout, err := positiveDuration("GRPC_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	dbPort, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	holdTTL, err := positiveDuration("HOLD_TTL")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Server: Server{
			Port: serverPort,
		},
		Clients: Clients{
			RoomAddr:    roomAddr,
			BookingAddr: bookingAddr,
			Timeout:     clientTimeout,
		},
		DB: DB{
			Host:     os.Getenv("DB_HOST"),
			Port:     dbPort,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
		},
		Kafka: Kafka{
			Brokers: brokers,
		},
		Saga: Saga{
			HoldTTL: holdTTL,
		},
	}, nil
}

// required returns the env var key, or an error if it is empty.
func required(key string) (string, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return "", fmt.Errorf("missing %s", key)
	}
	return v, nil
}

// positiveDuration parses the env var key with time.ParseDuration ("3s", "15m") and requires it to be > 0.
func positiveDuration(key string) (time.Duration, error) {
	d, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s: %v, must be > 0", key, d)
	}
	return d, nil
}
