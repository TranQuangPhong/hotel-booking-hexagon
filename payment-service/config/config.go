package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server Server // REST: Stripe webhook
	GRPC   GRPC   // gRPC: CreateCheckoutSession
	DB     DB
	Kafka  Kafka
	Stripe Stripe
}

type Server struct {
	Port int
}

type GRPC struct {
	Port int
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

type Stripe struct {
	SecretKey     string // API key (sandbox: sk_test_...)
	WebhookSecret string // signing secret used to verify webhook requests (whsec_...)
	SuccessURL    string // where the hosted payment page redirects after payment; {BOOKING_ID} is replaced
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil {
		// Ignore because production may not have .env.
	}

	serverPort, err := strconv.Atoi(os.Getenv("SERVER_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}

	grpcPort, err := strconv.Atoi(os.Getenv("GRPC_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid GRPC_PORT: %w", err)
	}

	dbPort, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	// Fail fast: an empty webhook secret would reject every Stripe webhook at runtime,
	// and a hosted checkout session can't be created without a success URL.
	stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")
	stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	stripeSuccessURL := os.Getenv("STRIPE_SUCCESS_URL")
	if stripeSecretKey == "" || stripeWebhookSecret == "" || stripeSuccessURL == "" {
		return Config{}, fmt.Errorf("STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET and STRIPE_SUCCESS_URL are required")
	}

	return Config{
		Server: Server{
			Port: serverPort,
		},
		GRPC: GRPC{
			Port: grpcPort,
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
		Stripe: Stripe{
			SecretKey:     stripeSecretKey,
			WebhookSecret: stripeWebhookSecret,
			SuccessURL:    stripeSuccessURL,
		},
	}, nil
}
