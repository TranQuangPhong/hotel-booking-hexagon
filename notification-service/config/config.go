package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// v1 is a Kafka consumer only: no HTTP server, no database.
// Later: MongoDB notification log (G9), SES (phase 2).
type Config struct {
	Kafka Kafka
}

type Kafka struct {
	Brokers []string
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil {
		// Ignore because production may not have .env.
	}

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return Config{
		Kafka: Kafka{
			Brokers: brokers,
		},
	}, nil
}
