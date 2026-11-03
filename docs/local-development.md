# Local development

The trigger-builder frontend is in `frontend`. The React prototype uses demo data and browser-local draft storage; it can be explored independently of the backend. See [frontend setup](../frontend/README.md).

From the repository root, build and start the frontend and backend with PostgreSQL and Kafka:

```sh
docker compose up -d --build --wait
docker compose logs -f backend
```

Open the frontend at `http://localhost:3000`. To start only the frontend, use `docker compose up -d --build --wait frontend`.

For frontend development with hot reload, run `cd frontend`, `npm ci`, then `npm run dev` in a separate shell. Vite prints its local URL, normally `http://127.0.0.1:5173`.

The `kafka-init` service creates `social.inbound` with three partitions before the backend starts. The backend uses `backend/config.docker.yml`, mounted read-only as `/app/config.yml`, with Docker network addresses for PostgreSQL and Kafka.

To run the backend directly with Go instead, start only the dependencies and topic initialization, then run the service from `backend`:

```sh
docker compose up -d --wait postgres kafka-init
cd backend
go run ./cmd
```

The Go module, service code, and `config.yml` live in `backend`. The consumer reads `config.yml` from the working directory. To use another file, run `go run ./cmd -config /path/to/config.yml` from `backend`.

On SIGINT or SIGTERM, the consumer stops reading and processing records, then runs the registered shutdown hooks with a separate 10-second timeout. Kafka closes before PostgreSQL. Initialization failures also close resources already created.

The consumer applies embedded Goose migrations at startup. It reads one Kafka record at a time. For each external comment it inserts the five-part key into PostgreSQL with a single atomic statement. It commits the offset after a successful insert or a confirmed duplicate. Own comments are skipped. A decoding, database, or offset-commit failure stops the consumer. Inspect the error before restarting. A malformed record will stop this group again until the record is handled.

To try duplicate delivery, run the following in a second shell from the repository root. The Kafka record value is the version 1 JSON payload. The sample uses a JSON array as a demonstration key encoding.

```sh
event='{"type":"CommentReceived","schema_version":1,"platform":"instagram","account_id":"acc_42","occurred_at":"2026-11-03T09:15:00Z","data":{"post_id":"17890001","comment_id":"18050002","author_id":"730003","is_own_comment":false,"text":"pricing"}}'
key='["instagram","acc_42","730003"]'
printf '%s\t%s\n' "$key" "$event" "$key" "$event" | docker compose exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic social.inbound --property parse.key=true
docker compose exec -T postgres psql -U automation -c 'TABLE processed_comments'
```

On a fresh database the two deliveries produce one row. To run the concurrency test against the local database, run `TEST_DATABASE_URL='postgres://automation:local@127.0.0.1:25432/automation?sslmode=disable' go test -count=1 ./...` from `backend`. Stop the Go process with Ctrl+C, or the container with `docker compose stop backend`. Stop the full stack with `docker compose stop` from the repository root. Compose allows 15 seconds for the backend to stop, including its 10-second shutdown timeout.

## Generate database code

Use sqlc v1.31.1. From `backend`, run:

```sh
make sqlc
```

The configuration is in `backend/sqlc.yaml`. SQL queries live in `backend/internal/db/queries`, the schema comes from Goose migrations, and generated pgx/v5 packages live in `backend/internal/db/sqlcDB` by feature. Regenerate and include the generated files whenever queries or schema change. Docker builds use these generated files directly.
