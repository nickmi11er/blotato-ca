# Architecture

The backend matches comment triggers and executes durable automation sessions, including asynchronous actions, replies, and timers. The React editor uses the backend API for drafts, publication, account/post metadata, and templates. Local sends are recorded, with an HTTP adapter available for the messaging service.

| Document | Purpose | Implementation status |
| --- | --- | --- |
| [001 · Inbound comment events](001-inbound-comment-events.md) | Kafka payload, partition key, and event identity | Implemented |
| [002 · Trigger matching and caching](002-trigger-matching-and-cache.md) | Published rules, account/post cache, TTL, invalidation, and session admission | Implemented |
| [003 · Session execution and DM routing](003-session-execution-and-dm-routing.md) | State machine, durable jobs, reply/timer handling, and safe DM route caching | Implemented |

For running services and generating SQL code, see [local development](../local-development.md). The [frontend README](../../frontend/README.md) describes the API-backed workflow builder and local simulator.
