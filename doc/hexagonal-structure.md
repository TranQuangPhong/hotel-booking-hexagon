user-service/
├── cmd/
│   └── main.go                 # initialize (Manual DI Container)
├── config/
│   └── config.go               # Load environment variables (.env, yaml)
├── internal/
│   ├── user/                 # HEXAGON CORE (Do not import anything external)
│   │   ├── user.go             # Struct User (Domain Model / Entity)
│   │   ├── service.go          # Business logic, seperate by usecase if needed
│   │   ├── identity.go         # Port: IdentityService (Cognito adapter will impl this)
│   │   └── repository.go       # Port: (DB interface here)
│   │
│   └── adapter/                # Outside world (Specific technology application)
│       ├── handler/
│       │   ├── handler.go           # Receive HTTP requests
│       │   └── router.go            # Endpoints (Gin)
│       ├── postgres/
│       │   ├── client.go            # pgx/sql.DB connection pool
│       │   └── user_repository.go   # Implement Port.UserRepository
│       └── cognito/                 # AWS Cognito SDK (delete user, sync)
│           └── client.go            # Implement Port.IdentityService
├── go.mod
└── README.md


room-service/
├── cmd/
│   └── main.go
│       # Entry point
│       # Load config
│       # Create infrastructure clients
│       # Wire adapters into services
│       # Start HTTP/Kafka consumers
├── config/
│   └── config.go # Configuration loader
├── internal/
│   ├── room/                             ===== Business =====
│   │   ├── room.go             # Domain models
│   │   ├── inventory.go        # Domain models
│   │   ├── service.go          # Business logic,  seperate by usecase if needed
│   │   ├── repository.go       # Port: DB
│   │   └── publisher.go        # Port: EventPublisher
│   │
│   ├── event/
│   │   ├── booking_created.go
│   │   ├── room_reserved.go
│   │   └── room_released.go    # Event contracts
│   │
│   └── adapter/                          ===== Adapters =====
│       ├── handler/
│       │   ├── room_handler.go     # HTTP -> RoomService
│       │   └── router.go           # Gin
│       ├── kafka/
│       │   ├── client.go           # Kafka producer/consumer client
│       │   ├── consumer.go         # Kafka -> RoomService
│       │   └── producer.go         # implements room.EventPublisher
│       ├── postgres/
│       │   ├── client.go           # pgx/sql.DB pool
│       │   ├── room_repository.go  # implements room.Repository
│       │   └── cached_room_repository.go
│       │       # Decorator
│       │       # Implements room.Repository
│       │       # Cache -> Postgres fallback
│       └── redis/
│           └── client.go
├── go.mod
└── README.md
