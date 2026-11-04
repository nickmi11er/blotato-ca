# 001. Inbound comment events

[Architecture index](README.md) · [Trigger matching and cache](002-trigger-matching-and-cache.md)

Each social platform integration converts a new comment into a common event and publishes it to Kafka. The automation module uses the connected account, post, and comment text to find a matching rule.

## Message format

The Kafka record value is UTF-8 JSON. Version 1 requires every field shown below.

```json
{
  "type": "CommentReceived",
  "schema_version": 1,
  "platform": "instagram",
  "account_id": "acc_42",
  "occurred_at": "2026-11-03T09:15:00Z",
  "data": {
    "post_id": "17890001",
    "comment_id": "18050002",
    "author_id": "730003",
    "is_own_comment": false,
    "text": "Please send me the pricing"
  }
}
```

| Field | Meaning |
| --- | --- |
| `type` | `CommentReceived` represents the creation of a comment. Edits require a separate event type. |
| `schema_version` | Integer identifying the message format. Republishing a comment under a newer schema preserves its event identity. |
| `platform` | Agreed platform code, such as `instagram` or `facebook`. |
| `account_id` | Stable internal ID of the connected social account. |
| `occurred_at` | Time the user posted the comment, in RFC 3339 format. |
| `data.post_id`, `data.comment_id`, `data.author_id` | Platform-issued string IDs for the post, comment, and author. Different platforms may issue the same ID. |
| `data.is_own_comment` | The integration sets this to true when the connected account authored the comment. The consumer skips these events so an automated public reply cannot trigger another run. |
| `data.text` | Original comment text used to evaluate the trigger. |

## Keys

| Purpose | Fields | Scope |
| --- | --- | --- |
| Kafka partitioning | `(platform, account_id, participant_id)` | All inbound events from one participant interacting with one connected account |
| Processing idempotency | `(platform, account_id, post_id, comment_id, type)` | One comment creation event |

### Partition key

For a comment, the participant is its author. For an inbound direct message (DM), the participant is its sender. In the example, the integration uses `730003` as the participant ID in the Kafka key. The payload already identifies the author.

To route a participant's comments and DMs to the same partition, producers must use the same topic, serialize the key to identical bytes, and use the same key-based partitioning strategy. Use the compact JSON array encoding produced by `shared/kafka.RoutingKey` (Go JSON string escaping), for example `["instagram","acc_42","730003"]`, and Java-compatible Murmur2 partitioning. The configured partition count is fixed at three locally; the consumer validates the count, key bytes, and partition. Changing the count requires a coordinated routing migration. If a platform issues different IDs for comment authors and DM senders, the integration needs a verified mapping to a common participant ID.

A consumer reading a partition sequentially sees records in the order Kafka appended them. An integration delay can cause a user's earlier action to arrive later. The [session and DM routing protocol](003-session-execution-and-dm-routing.md#dm-routing-cache-and-activation-barrier) uses ordered conversation activation and exclusive reply routing to make negative DM caching safe.

### Idempotency key

The store enforces uniqueness with a composite constraint on these five fields. The [matching pipeline](002-trigger-matching-and-cache.md#deduplicate-after-matching) records only matching events: the decision, pending sessions, and execution jobs commit together before acknowledging Kafka. Repeat-policy denials are also recorded.

Non-matches leave no durable decision, so redelivery after a rule change can produce a match. Failed cache loads and admission transactions retry the same record with backoff. Invalid payloads stop the consumer without acknowledging the record.

For example, two comments with identical text from the same author on the same post share a partition key. Their distinct comment IDs give them separate idempotency keys. Redelivering either comment preserves both of its keys. Comment text, delivery time, schema version, and per-publication UUIDs are excluded from the idempotency key.

## Direct messages and activation

DMs use the same topic and routing key. `participant_id` must identify the same person as the comment author. `reply_to_message_id` is optional; all other fields below are required. Own messages and empty text are ignored.

```json
{
  "type": "MessageReceived",
  "schema_version": 1,
  "platform": "instagram",
  "account_id": "acc_42",
  "occurred_at": "2026-11-03T09:16:00Z",
  "data": {
    "participant_id": "730003",
    "conversation_id": "thread_1",
    "message_id": "dm_1",
    "is_own_message": false,
    "text": "My email is alex@example.com"
  }
}
```

The backend also publishes `ActivateConversation` on this stream: a flat object with `type`, `schema_version: 1`, `platform`, `account_id`, `participant_id`, `session_id`, and `epoch`. It reserves a route before the question can be sent. Social integrations publish external comments and DMs; activation is an internal control event.
