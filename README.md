# Cutter Project

A high-performance, secure Go web application built with Fiber framework, featuring clean architecture, dependency injection, and robust security measures.

## Features

- Clean Architecture with Dependency Injection
- RESTful API design
- PostgreSQL with optimized connection pooling
- Redis for caching with optimized client settings
- Structured logging with Zap
- Graceful shutdown
- Environment-based configuration
- Database migrations
- JWT-based authentication with refresh tokens
- Comprehensive error handling
- Security measures (CORS, rate limiting, authentication middleware)
- Performance optimizations

## Project Structure

```
├── cmd/                    # Application entry points
│   └── main.go            # Main application
├── internal/              # Private application code
│   ├── config/            # Configuration setup
│   │   ├── app.go         # App configuration (moved to container)
│   │   ├── database.go    # Database configuration
│   │   ├── fiber.go       # Fiber configuration
│   │   └── zap.go         # Logger configuration
│   ├── container/         # Dependency injection container
│   │   └── container.go   # Container implementation
│   ├── delivery/          # Delivery layer (HTTP, gRPC, etc.)
│   │   └── http/          # HTTP delivery
│   │       ├── middleware/ # HTTP middlewares
│   │       │   ├── auth_middleware.go      # Authentication middleware
│   │       │   ├── cors_middleware.go      # CORS middleware
│   │       │   └── rate_limiter_middleware.go # Rate limiting middleware
│   │       ├── route/     # Route configuration
│   │       │   └── route.go # Route setup
│   │       └── user_controller.go # User controller
│   ├── exception/         # Error handling
│   │   ├── error_handler.go # Error handler implementation
│   │   └── errors.go      # Custom error types
│   ├── model/             # Domain models
│   │   └── user.go        # User model
│   ├── repository/        # Data access layer
│   │   └── user_repository.go # User repository
│   └── usecase/           # Business logic layer
│       └── user_usecase.go # User use case
├── db/                    # Database migrations
│   └── 001_create_users_table.sql # Users table migration
├── .env.example           # Environment variables template
├── docker-compose.yml     # Docker configuration
├── Dockerfile             # Docker image configuration
├── .gitignore             # Git ignore rules
├── go.mod                 # Go modules
├── go.sum                 # Go modules checksums
├── Makefile               # Build automation
└── README.md              # This file
```

## Getting Started

### Prerequisites

- Go 1.23 or higher
- PostgreSQL 12 or higher
- Redis 6 or higher
- Docker (optional)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/cutterproject.git
   cd cutterproject
   ```

2. Copy the environment variables template:
   ```bash
   cp .env.example .env
   ```

3. Edit the `.env` file with your configuration:
   ```bash
   # Server Configuration
   GO_SERVER=:8080
   
   # Database Configuration
   POSTGRES_URL=postgres://username:password@localhost:5432/dbname?sslmode=disable
   
   # Redis Configuration
   REDIS_URL=redis://localhost:6379
   
   # Logging Configuration
   LOG_LEVEL=info
   ```

4. Install dependencies:
   ```bash
   go mod download
   ```

5. Run database migrations:
   ```bash
   # Using psql
   psql $POSTGRES_URL -f db/migrations/001_create_users_table.sql
   ```

6. Run the application:
   ```bash
   go run cmd/main.go
   ```

### Using Docker

1. Build and run with Docker Compose:
   ```bash
   docker-compose up --build
   ```

## API Endpoints

### Public Endpoints

- `POST /api/users/register` - Register a new user
- `POST /api/users/login` - User login

### Protected Endpoints (Authentication Required)

- `GET /api/user/:id` - Get user by ID
- `PUT /api/user/:id` - Update user information
- `PUT /api/user/:id/password` - Change user password
- `DELETE /api/user/:id` - Delete user account

### Health Check

- `GET /api/health` - Health check endpoint

## Architecture

This project follows a clean architecture pattern with the following layers:

1. **Delivery Layer**: Handles HTTP requests and responses
2. **Use Case Layer**: Contains business logic
3. **Repository Layer**: Handles data access
4. **Model Layer**: Contains data structures

### Dependency Injection

The project uses a dependency injection pattern to manage dependencies. The `container` package is responsible for creating and managing all dependencies.

### Error Handling

The project has a comprehensive error handling system with custom error types and a centralized error handler.

### Security

The project implements several security measures:

- JWT-based authentication
- CORS middleware
- Rate limiting
- Input validation
- Secure password hashing

### Performance

The project includes several performance optimizations:

- Optimized database connection pooling
- Optimized Redis client settings
- Fiber framework optimizations
- Efficient error handling

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o bin/cutterproject cmd/main.go
```

### Using Makefile

```bash
# Build the application
make build

# Run the application
make run

# Run tests
make test

# Clean build artifacts
make clean
```

## Configuration

The application uses environment variables for configuration. See `.env.example` for all available options.

## Security

- Passwords are hashed using bcrypt
- Sensitive data is not exposed in API responses
- Input validation is performed on all endpoints
- CORS is configured for cross-origin requests
- JWT-based authentication with refresh tokens
- Rate limiting to prevent abuse

## Performance

- Connection pooling for PostgreSQL with optimized settings
- Redis caching for frequently accessed data with optimized client settings
- Efficient JSON serialization with Sonic
- Configurable timeouts and limits
- Fiber framework optimizations

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.