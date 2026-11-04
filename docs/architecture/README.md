# Architecture

The backend matches comment triggers and atomically admits automation sessions. Workflow execution and frontend API integration remain pending.

| Document | Purpose | Implementation status |
| --- | --- | --- |
| [001 · Inbound comment events](001-inbound-comment-events.md) | Kafka payload, partition key, and event identity | Implemented |
| [002 · Trigger matching and caching](002-trigger-matching-and-cache.md) | Published rules, account/post cache, TTL, invalidation, and session admission | Implemented; workflow execution pending |

For running services and generating SQL code, see [local development](../local-development.md). The [frontend README](../../frontend/README.md) describes the browser-local workflow builder and simulator.
