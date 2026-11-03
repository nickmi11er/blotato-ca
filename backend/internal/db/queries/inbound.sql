-- name: RecordComment :execrows
INSERT INTO processed_comments (platform, account_id, post_id, comment_id, event_type)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (platform, account_id, post_id, comment_id, event_type) DO NOTHING;
