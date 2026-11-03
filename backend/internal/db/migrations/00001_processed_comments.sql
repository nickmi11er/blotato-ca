-- +goose Up
CREATE TABLE processed_comments (
    platform text NOT NULL,
    account_id text NOT NULL,
    post_id text NOT NULL,
    comment_id text NOT NULL,
    event_type text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, account_id, post_id, comment_id, event_type)
);

-- +goose Down
DROP TABLE processed_comments;
