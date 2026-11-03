# 001. Inbound comment events

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

To route a participant's comments and DMs to the same partition, producers must use the same topic, serialize the key to identical bytes, and use the same key-based partitioning strategy. This assumes a stable partition count. The implementation must specify a shared key encoding. If a platform issues different IDs for comment authors and DM senders, the integration needs a verified mapping to a common participant ID.

A consumer reading a partition sequentially sees records in the order Kafka appended them. An integration delay can cause a user's earlier action to arrive later. If the participant has multiple automation runs, identifying which run a reply belongs to is a separate decision.

### Idempotency key

The store can enforce uniqueness with a composite constraint on the five key fields. The consumer records the key and the processing decision in one transaction, then commits the Kafka offset after the transaction succeeds. On redelivery, the existing key prevents a second run. If the transaction rolls back, the consumer can retry the entire operation.

For example, two comments with identical text from the same author on the same post share a partition key. Their distinct comment IDs give them separate idempotency keys. Redelivering either comment preserves both of its keys. Comment text, delivery time, schema version, and per-publication UUIDs are excluded from the idempotency key.
