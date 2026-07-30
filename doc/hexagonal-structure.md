room-service/
├── cmd/
│   └── room-service/
│       └── main.go
│           # Entry point
│           # Load config
│           # Create infrastructure clients
│           # Wire adapters into services
│           # Start HTTP/Kafka consumers
├── config/
│   └── config.go # Configuration loader
├── internal/
│   ├── domain/                             ===== Business =====
│   │   ├── model.go # Domain model
│   │   ├── service.go # Business logic
│   │   ├── repository.go # Port: Repository
│   │   └── publisher.go # Port: EventPublisher
│   │
│   ├── event/
│   │   ├── booking_created.go
│   │   ├── room_reserved.go
│   │   └── room_released.go # Event contracts
│   │
│   ├── adapter/                          ===== Adapters =====
│   │   ├── http/
│   │   │   └── room_handler.go # HTTP -> RoomService
│   │   ├── kafka/
│   │   │   ├── consumer.go # Kafka -> RoomService
│   │   │   └── producer.go # implements room.EventPublisher
│   │   └── postgres/
│   │       ├── room_repository.go # implements room.Repository
│   │       └── cached_room_repository.go
│   │           # Decorator
│   │           # Implements room.Repository
│   │           # Cache -> Postgres fallback
│   │
│   └── infrastructure/                  ===== Shared Infrastructure =====
│       ├── postgres/
│       │   └── client.go # pgx/sql.DB pool
│       ├── redis/
│       │   └── client.go # Redis client
│       └── kafka/
│           └── client.go # Kafka producer/consumer client
├── go.mod
└── README.md
