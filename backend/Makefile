DB_DSN=postgres://postgres:postgres@localhost:5432/vyborok?sslmode=disable
TEST_DB_DSN=postgres://postgres:postgres@localhost:5432/vyborok_test?sslmode=disable

.PHONY: run migrate-up migrate-down migrate-version migrate-create test test-repo test-db-create test-db-migrate test-db-reset gen

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


test:
	go test ./... -race

test-repo:
	TEST_DATABASE_URL="$(TEST_DB_DSN)" go test ./internal/repository/... -v

test-db-create:
	docker exec -it vyborok-postgres psql -U postgres -c "CREATE DATABASE vyborok_test;" || true

test-db-migrate:
	migrate -path db/migrations -database "$(TEST_DB_DSN)" up

test-db-reset:
	docker exec -it vyborok-postgres psql -U postgres -c "DROP DATABASE IF EXISTS vyborok_test;"
	$(MAKE) test-db-create
	$(MAKE) test-db-migrate


gen:
	cd http/proto && go generate
