package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server Server
	DB     DB
	Kafka  Kafka
}

type Server struct {
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

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil {
		// Ignore because production may not have .env.
	}

	serverPort, err := strconv.Atoi(os.Getenv("SERVER_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}

	dbPort, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return Config{
		Server: Server{
			Port: serverPort,
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
	}, nil
}
