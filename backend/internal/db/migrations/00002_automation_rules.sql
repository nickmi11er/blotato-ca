-- +goose Up
CREATE TABLE workspaces (id text PRIMARY KEY);
CREATE TABLE connected_accounts (
    id text PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES workspaces(id),
    platform text NOT NULL,
    UNIQUE (tenant_id, id)
);
CREATE TABLE automations (
    tenant_id text NOT NULL REFERENCES workspaces(id),
    id text NOT NULL,
    draft jsonb NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    revision bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, id)
);
CREATE TABLE automation_versions (
    tenant_id text NOT NULL,
    automation_id text NOT NULL,
    revision bigint NOT NULL,
    document jsonb NOT NULL,
    PRIMARY KEY (tenant_id, automation_id, revision)
);
-- Versions and session history survive automation deletion.
CREATE TABLE account_rule_sets (
    account_id text PRIMARY KEY REFERENCES connected_accounts(id),
    generation bigint NOT NULL DEFAULT 0
);
CREATE TABLE trigger_bindings (
    tenant_id text NOT NULL,
    account_id text NOT NULL,
    automation_id text NOT NULL,
    revision bigint NOT NULL,
    post_scope text NOT NULL CHECK (post_scope IN ('all', 'selected')),
    PRIMARY KEY (tenant_id, account_id, automation_id),
    FOREIGN KEY (tenant_id, account_id) REFERENCES connected_accounts(tenant_id, id),
    FOREIGN KEY (tenant_id, automation_id) REFERENCES automations(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, automation_id, revision) REFERENCES automation_versions(tenant_id, automation_id, revision)
);
CREATE INDEX trigger_bindings_scope ON trigger_bindings (account_id, post_scope);
CREATE TABLE trigger_posts (
    tenant_id text NOT NULL,
    account_id text NOT NULL,
    post_id text NOT NULL,
    automation_id text NOT NULL,
    PRIMARY KEY (account_id, post_id, automation_id),
    FOREIGN KEY (tenant_id, account_id, automation_id) REFERENCES trigger_bindings(tenant_id, account_id, automation_id) ON DELETE CASCADE
);
CREATE TABLE rule_changes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id text NOT NULL,
    generation bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE automation_sessions (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    automation_id text NOT NULL,
    revision bigint NOT NULL,
    account_id text NOT NULL,
    participant_id text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'waiting', 'completed', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, automation_id, revision) REFERENCES automation_versions(tenant_id, automation_id, revision)
);
CREATE INDEX session_participant ON automation_sessions (tenant_id, automation_id, account_id, participant_id);
CREATE TABLE admission_locks (
    tenant_id text NOT NULL,
    automation_id text NOT NULL,
    account_id text NOT NULL,
    participant_id text NOT NULL,
    PRIMARY KEY (tenant_id, automation_id, account_id, participant_id)
);
CREATE TABLE comment_decisions (
    platform text NOT NULL,
    account_id text NOT NULL,
    post_id text NOT NULL,
    comment_id text NOT NULL,
    event_type text NOT NULL,
    outcomes jsonb NOT NULL,
    PRIMARY KEY (platform, account_id, post_id, comment_id, event_type),
    FOREIGN KEY (platform, account_id, post_id, comment_id, event_type)
        REFERENCES processed_comments(platform, account_id, post_id, comment_id, event_type)
);
CREATE TABLE session_outbox (
    session_id text PRIMARY KEY REFERENCES automation_sessions(id),
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE session_outbox, comment_decisions, admission_locks, automation_sessions;
DROP TABLE rule_changes, trigger_posts, trigger_bindings, account_rule_sets;
DROP TABLE automation_versions, automations, connected_accounts, workspaces;
