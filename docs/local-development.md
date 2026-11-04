# Local development

See the [architecture index](architecture/README.md) for the inbound contract, trigger matching/cache, and session execution design. The commands below describe the currently implemented services.

The trigger-builder frontend is in `frontend`. The React prototype uses demo data and browser-local draft storage; it can be explored independently of the backend. See [frontend setup](../frontend/README.md).

From the repository root, build and start the frontend and backend with PostgreSQL and Kafka:

```sh
docker compose up -d --build --wait
docker compose logs -f backend
```

Open the frontend at `http://localhost:3000`. To start only the frontend, use `docker compose up -d --build --wait frontend`.

For frontend development with hot reload, run `cd frontend`, `npm ci`, then `npm run dev` in a separate shell. Vite prints its local URL, normally `http://127.0.0.1:5173`.

The `kafka-init` service creates `social.inbound` with three partitions and `automation.rules` with one partition before the backend starts. The backend uses `backend/config.docker.yml`, mounted read-only as `/app/config.yml`, with Docker network addresses for PostgreSQL and Kafka.

To run the backend directly with Go instead, start only the dependencies and topic initialization, then run the service from `backend`:

```sh
docker compose up -d --wait postgres kafka-init
cd backend
go run ./cmd
```

The Go module, service code, and `config.yml` live in `backend`. The consumer reads `config.yml` from the working directory. To use another file, run `go run ./cmd -config /path/to/config.yml` from `backend`.

On SIGINT or SIGTERM, the consumer stops reading and processing records, then runs the registered shutdown hooks with a separate 10-second timeout. Kafka closes before PostgreSQL. Initialization failures also close resources already created.

The backend applies embedded Goose migrations at startup. It matches comments against a bounded local cache, then atomically records matching decisions, repeat-policy admission, sessions, and execution jobs. Workers execute published workflows, persist waits, and resume them from DMs or scheduled jobs. Fresh non-matches and own comments require no database write. Infrastructure failures retry without acknowledging the record; malformed events stop the consumer.

## Publish a trigger

The internal API listens on `127.0.0.1:8080`. The checked-in bearer token is for local development; a trusted gateway/integration holds this credential. The frontend still uses browser-local storage. Publication validates and compiles the workflow. Email templates are embedded snapshots for now; template CRUD and frontend API integration are separate work.

Register the account, save a draft, and publish it before sending a comment:

```sh
api=http://127.0.0.1:8080/v1/workspaces/demo
auth='Authorization: Bearer local-development-internal-token'
curl --fail-with-body -X PUT "$api/accounts/acc_42" -H "$auth" -H 'Content-Type: application/json' -d '{"platform":"instagram"}'
curl --fail-with-body -X PUT "$api/automations/pricing" -H "$auth" -H 'Content-Type: application/json' --data-binary @- <<'JSON'
{
  "name": "Pricing",
  "priority": 10,
  "trigger": {
    "schema_version": 1,
    "scopes": [{"account_id": "acc_42", "posts": {"mode": "all"}}],
    "comment": {"mode": "keywords", "operator": "whole_word", "logic": "any", "keywords": ["pricing"], "exclusions": [], "case_sensitive": false},
    "repeat": {"mode": "skip_active"}
  },
  "workflow": {
    "schema_version": 1,
    "templates": {"pricing": {"revision": 1, "subject": "Your pricing link", "body": "See https://example.com/pricing"}},
    "steps": [
      {"id": "intro", "kind": "parallel", "actions": [
        {"id": "reply", "kind": "reply", "text": "Sent you a DM!"},
        {"id": "ask", "kind": "dm", "text": "What is your email address?"}
      ]},
      {"id": "capture", "kind": "wait_reply", "prompt_node_id": "ask", "expected": "email", "timeoutHours": 24, "invalidBehavior": "retry", "retryMessage": "Please send one email address.", "maxAttempts": 3, "timeoutBehavior": "stop"},
      {"id": "send", "kind": "email", "templateId": "pricing"}
    ]
  }
}
JSON
curl --fail-with-body -X POST "$api/automations/pricing/publish" -H "$auth"
```

`GET` on the automation returns its draft and publication status. `POST /disable` removes live bindings; `DELETE` on the automation deletes it while preserving version/session history. `PUT` changes only the draft. Publication propagates asynchronously; allow the outbox notification to reach consumers before testing a newly published rule.

## Process a comment

To try duplicate delivery, run the following in a second shell from the repository root. The Kafka record value is the version 1 JSON payload. The compact JSON array key and Java producer's Murmur2 partitioner satisfy the routing contract.

```sh
event='{"type":"CommentReceived","schema_version":1,"platform":"instagram","account_id":"acc_42","occurred_at":"2026-11-03T09:15:00Z","data":{"post_id":"17890001","comment_id":"18050002","author_id":"730003","is_own_comment":false,"text":"pricing"}}'
key='["instagram","acc_42","730003"]'
printf '%s\t%s\n' "$key" "$event" "$key" "$event" | docker compose exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic social.inbound --property parse.key=true
docker compose exec -T postgres psql -U automation -c 'TABLE comment_decisions'
docker compose exec -T postgres psql -U automation -c 'TABLE automation_sessions'
```

On a fresh database, duplicate deliveries produce one decision and one session. The workers activate the conversation, record the public reply and question, and wait for an email. Use a new comment ID when repeating the example; an active reply-capable session also blocks another launch for the same participant.

## Resume from a DM

The default `delivery.mode: recording` writes requests to `recorded_deliveries` without sending real messages. Inspect it to see the prompt, then send the response with the same routing key:

```sh
docker compose exec -T postgres psql -U automation -c 'TABLE recorded_deliveries'
event='{"type":"MessageReceived","schema_version":1,"platform":"instagram","account_id":"acc_42","occurred_at":"2026-11-03T09:16:00Z","data":{"participant_id":"730003","conversation_id":"thread_1","message_id":"dm_1","is_own_message":false,"text":"My email is alex@example.com"}}'
key='["instagram","acc_42","730003"]'
printf '%s\t%s\n' "$key" "$event" | docker compose exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic social.inbound --property parse.key=true
docker compose exec -T postgres psql -U automation -c 'SELECT id, status, context, finish_reason FROM automation_sessions'
docker compose exec -T postgres psql -U automation -c 'TABLE recorded_deliveries'
```

After the workers process the reply, the session becomes `completed` and the rendered email appears in `recorded_deliveries`. Add a `delay` step with `minutes: 60` before `send` to defer it for an hour. A process restart preserves the timer.

Inspect one session with `GET /v1/workspaces/demo/sessions/{id}`, using the same bearer token. State, steps, jobs, and transition history are returned together. Cancellation, unknown-outcome resolution, and real HTTP delivery configuration are described in [session execution](architecture/003-session-execution-and-dm-routing.md#delivery-boundary-and-operations).

Stop the Go process with Ctrl+C, or use `docker compose stop backend`. Compose allows 15 seconds for shutdown.

## Verify

From `backend`, run the integration tests with local dependencies:

```sh
TEST_DATABASE_URL='postgres://automation:local@127.0.0.1:25432/automation?sslmode=disable' TEST_KAFKA_BROKER='127.0.0.1:29094' go test -race -count=1 ./...
go vet ./...
```

Each PostgreSQL test creates and removes its own schema; the Kafka execution test creates an isolated topic/group and checks rebalance with two consumers. Without these environment variables, external integration tests are skipped. Go and React run the same Unicode matching cases from `frontend/src/feature/automation/model/testdata/matching.json`; run `npm run check` in `frontend` as well.

## Generate database code

Use sqlc v1.31.1. From `backend`, run:

```sh
make sqlc
```

The configuration is in `backend/sqlc.yaml`. SQL queries live in `backend/internal/db/queries`, the schema comes from Goose migrations, and generated pgx/v5 packages live in `backend/internal/db/sqlcDB` by feature. Regenerate and include the generated files whenever queries or schema change. Docker builds use these generated files directly.
