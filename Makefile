APP=walker-tui
DSN=postgres://dev:dev@localhost:5432/zeropoint?sslmode=disable

.PHONY: run build migrate-up migrate-down db-up db-down export enums

build:
	go build -o $(APP) ./cmd/zeropoint

# Legacy alias
zeropoint: build
	@echo "Built $(APP) binary (legacy target)"

run: build
	./$(APP) --dsn $(DSN)

migrate-up: build
	./$(APP) --dsn $(DSN) migrate up

migrate-down: build
	./$(APP) --dsn $(DSN) migrate down

db-up:
	docker compose up -d

db-down:
	docker compose down

export: run
	# placeholder target to ensure binary runs first

enums:
	go generate ./internal/engine
