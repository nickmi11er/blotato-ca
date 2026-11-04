.PHONY: sqlc integration-up test-integration

TEST_DATABASE_URL ?= postgres://automation:local@127.0.0.1:25432/automation?sslmode=disable
TEST_KAFKA_BROKER ?= 127.0.0.1:29094
export TEST_DATABASE_URL TEST_KAFKA_BROKER

sqlc:
	sqlc generate

integration-up:
	docker compose -f ../compose.yaml up -d --wait postgres kafka

test-integration:
	@test -n "$(TEST_DATABASE_URL)" && test -n "$(TEST_KAFKA_BROKER)" || (echo "Both test dependency addresses are required"; exit 1)
	go test -race -count=1 -timeout=3m ./...
