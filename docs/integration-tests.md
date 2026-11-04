# Integration tests

```sh
make -C backend integration-up test-integration
```

The target runs all Go tests with the race detector and disables result caching. Override `TEST_DATABASE_URL` / `TEST_KAFKA_BROKER` for other dependencies. Plain `go test ./...` skips external integration tests if their environment variables are absent; the Make target supplies both and fails if dependencies are unavailable.

Each test applies real migrations to a private PostgreSQL schema. Kafka tests create and delete private topics/groups; they do not consume application topics or modify demo data. External sends use a local HTTP server or the recording adapter, never social/email providers.

## Main cases

| Boundary | Expected behavior |
| --- | --- |
| API → rules outbox → two replicas → Kafka → workers → HTTP delivery | Drafts do not match; publication invalidates both negative caches. Selected posts, whole words, exclusions, own comments and repeat policy filter admission; duplicate events do not duplicate effects. |
| Core pricing scenario | On two arbitrary posts: duplicate pricing comments produce a public reply and DM prompt; duplicate invalid replies re-ask once; a valid email resumes the session and sends one DM containing the link. No email template or email send is needed. |
| Published version and template | Editing, republishing, disabling or deleting an automation leaves an existing session on its original graph/template. Disabled rules admit no new sessions. |
| Reply capture | An unrelated DM warms the negative route cache; activation replaces it before sending the question. A reply arriving before the HTTP receipt still completes the session. Invalid and valid DM duplicates are consumed once. |
| Workspace/account isolation | Identical participant/comment IDs on Instagram and Facebook remain separate. Cross-workspace reads/cancellation are rejected. Late DMs cannot resume cancelled sessions. |
| Ambiguous HTTP outcome | A provider sends once but loses the receipt; subsequent attempts return 503, 429 or a rejection. Retries retain the same key; exhausting them leaves `unknown`, which can be reconciled and resumed. |
| Lost result commit | A send succeeds before a simulated worker crash. Lease recovery preserves uncertainty even when later attempts get 429; explicit reconciliation still resolves the job. |
| Invalid receipt | Trailing JSON, trailing garbage and responses exceeding 64 KiB cannot confirm a send. |
| Definite rejection / parallel actions | Exhausted retries fail the session only when the integration guarantees no send. A successful sibling is not resent. |
| Cancellation during HTTP send | An in-flight send may finish; its late receipt cannot resume the cancelled session or enqueue the next action. |
| Recovery and concurrency | Existing feature tests cover concurrent admission/workers, rollback, lease fencing, restart, reply/timeout ordering, durable delays, and Kafka rebalance. |

The complete pipeline is in `backend/internal/integration`; focused PostgreSQL/HTTP regressions are in `internal/feature/execution/delivery_integration_test.go`. Feature integration tests remain beside their implementation.

Pipeline tests run the real rules publisher, consumers and workers. They wait for each replica's observed generation; rule TTL exceeds assertion deadlines, so expiry cannot hide a missing invalidation. Failure tests move retry timestamps directly in the isolated database to avoid waiting through backoff; claiming, HTTP delivery and result commits still use production code. These are correctness tests, not throughput benchmarks or real-provider certification.

## Browser verification — 2026-11-04

Chrome/Playwright against the rebuilt Compose frontend and backend, with real PostgreSQL and Kafka; no API mocks. Desktop 1440 px and mobile 390 px were inspected.

| Checked through the UI | Result |
| --- | --- |
| Create/rename/save/reload drafts; publish, edit, pause and republish | Passed; invalid/incomplete flows cannot publish |
| Selected posts, whole-word/exact/substring/any-comment matching and exclusions | Passed in the editor/simulator; post scope, whole words and exclusions also checked with Kafka events |
| Empty parallel group, independent actions, menu outside-click, duplicate/undo | Passed |
| Email capture, invalid reply, delay, past scheduled date, final timeout DM and separate timeout branch | Passed in the simulator |
| Template create/edit/reload, reference protection and deletion | Passed |
| Concurrent tabs and offline save/retry | Conflict returns 409; network failure preserves edits and allows retry |
| Automation deletion: cancel, confirm, reload | Passed; QA drafts/templates removed, original demo retained |
| Selected automation after reload and returning from templates | Found and fixed: identity now persists in the URL; missing IDs show the list instead of an unrelated editor |

A flow configured and published in the browser also ran through Kafka: duplicate comments produced one session; duplicate invalid DMs produced one re-ask; a valid email resumed the session after a real one-minute delay and rendered the email. A second run stopped at three invalid replies; a paused flow admitted no new session. Outbound requests used the configured recording adapter, not real social/email delivery. Timeout branches were exercised with the simulator's virtual clock, not an hour-long live wait. No uncaught browser JavaScript errors were observed.
