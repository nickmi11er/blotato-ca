-- +goose Up
ALTER TABLE automations ADD COLUMN draft_version bigint NOT NULL DEFAULT 1,
    ADD COLUMN published_draft_version bigint NOT NULL DEFAULT 0,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE connected_accounts ADD COLUMN name text NOT NULL DEFAULT '',
    ADD COLUMN handle text NOT NULL DEFAULT '', ADD COLUMN can_send_dm boolean NOT NULL DEFAULT true;
UPDATE connected_accounts SET name=platform,handle=id,can_send_dm=(platform<>'youtube');
CREATE TABLE social_posts (
    tenant_id text NOT NULL,
    account_id text NOT NULL,
    id text NOT NULL,
    title text NOT NULL,
    published_at timestamptz NOT NULL,
    PRIMARY KEY(account_id,id),
    FOREIGN KEY(tenant_id,account_id) REFERENCES connected_accounts(tenant_id,id)
);
CREATE INDEX social_posts_workspace ON social_posts(tenant_id,published_at DESC,account_id,id);
CREATE TABLE email_templates (
    tenant_id text NOT NULL REFERENCES workspaces(id),
    id text NOT NULL,
    name text NOT NULL,
    subject text NOT NULL,
    body text NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    PRIMARY KEY(tenant_id,id)
);
-- +goose Down
DROP TABLE email_templates,social_posts;
ALTER TABLE connected_accounts DROP COLUMN name,DROP COLUMN handle,DROP COLUMN can_send_dm;
ALTER TABLE automations DROP COLUMN draft_version,DROP COLUMN published_draft_version,DROP COLUMN updated_at;
