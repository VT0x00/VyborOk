DB_DSN=postgres://postgres:postgres@localhost:5432/vyborok?sslmode=disable

.PHONY: run migrate-up migrate-down migrate-version migrate-create

run:
	go run main.go

migrate-up:
	migrate -path db/migrations -database "$(DB_DSN)" up

migrate-down:
	migrate -path db/migrations -database "$(DB_DSN)" down 1

migrate-version:
	migrate -path db/migrations -database "$(DB_DSN)" version

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir db/migrations -seq $$name
