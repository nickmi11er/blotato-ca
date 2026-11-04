-- +goose Up
ALTER TABLE automation_versions ADD COLUMN execution_plan jsonb;
ALTER TABLE automation_sessions DROP CONSTRAINT automation_sessions_status_check;
ALTER TABLE automation_sessions ADD CONSTRAINT automation_sessions_status_check CHECK (status IN ('pending','running','waiting','completed','stopped','failed','cancelled'));
ALTER TABLE automation_sessions
    ADD COLUMN current_node_id text NOT NULL DEFAULT '',
    ADD COLUMN context jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN origin jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN version bigint NOT NULL DEFAULT 0,
    ADD COLUMN finish_reason text NOT NULL DEFAULT '',
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE automation_sessions s SET origin = o.payload->'comment' FROM session_outbox o WHERE o.session_id=s.id;
CREATE TABLE conversation_owners (
    tenant_id text NOT NULL,
    account_id text NOT NULL,
    participant_id text NOT NULL,
    session_id text REFERENCES automation_sessions(id),
    epoch bigint NOT NULL DEFAULT 0,
    next_sequence bigint NOT NULL DEFAULT 0,
    active boolean NOT NULL DEFAULT false,
    PRIMARY KEY (account_id,participant_id),
    FOREIGN KEY (tenant_id,account_id) REFERENCES connected_accounts(tenant_id,id)
);
CREATE TABLE session_steps (
    session_id text NOT NULL REFERENCES automation_sessions(id),
    node_id text NOT NULL,
    parent_node_id text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','armed','waiting','succeeded','stopped','failed','cancelled')),
    wait_kind text NOT NULL DEFAULT '',
    listen_from_sequence bigint NOT NULL DEFAULT 0,
    deadline_at timestamptz,
    invalid_attempts integer NOT NULL DEFAULT 0,
    output jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY (session_id,node_id)
);
CREATE TABLE session_inbox (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id text NOT NULL,
    participant_id text NOT NULL,
    conversation_id text NOT NULL,
    provider_message_id text NOT NULL,
    event_type text NOT NULL DEFAULT 'MessageReceived',
    session_id text NOT NULL REFERENCES automation_sessions(id),
    conversation_epoch bigint NOT NULL,
    sequence bigint NOT NULL,
    accepted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    payload jsonb NOT NULL,
    consumed_at timestamptz,
    UNIQUE (account_id,conversation_id,provider_message_id,event_type)
);
CREATE INDEX inbox_pending ON session_inbox (session_id,sequence) WHERE consumed_at IS NULL;
CREATE TABLE execution_jobs (
    id text PRIMARY KEY,
    session_id text NOT NULL REFERENCES automation_sessions(id),
    node_id text NOT NULL DEFAULT '',
    logical_key text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('advance','deliver','wake','activate')),
    status text NOT NULL DEFAULT 'ready' CHECK (status IN ('ready','leased','done','failed','unknown','cancelled')),
    run_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    lease_token bigint NOT NULL DEFAULT 0,
    attempt integer NOT NULL DEFAULT 0,
    payload jsonb NOT NULL DEFAULT '{}',
    result jsonb NOT NULL DEFAULT '{}',
    UNIQUE (session_id,logical_key)
);
CREATE INDEX jobs_ready ON execution_jobs (run_at,id) WHERE status='ready';
CREATE INDEX jobs_leased ON execution_jobs (lease_until,id) WHERE status='leased';
CREATE INDEX jobs_session ON execution_jobs (session_id,node_id);
INSERT INTO execution_jobs(id,session_id,logical_key,kind,payload)
SELECT session_id||'/start',session_id,'start','advance',payload FROM session_outbox;
DROP TABLE session_outbox;
CREATE TABLE session_transitions (
    session_id text NOT NULL REFERENCES automation_sessions(id),
    version bigint NOT NULL,
    node_id text NOT NULL,
    event_id text NOT NULL,
    from_state text NOT NULL,
    to_state text NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (session_id,version)
);
-- Explicit local integration substitute. Real delivery uses the HTTP adapter.
CREATE TABLE recorded_deliveries (
    idempotency_key text PRIMARY KEY,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- +goose Down
CREATE TABLE session_outbox (
    session_id text PRIMARY KEY REFERENCES automation_sessions(id),
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO session_outbox(session_id,payload) SELECT session_id,payload FROM execution_jobs WHERE logical_key='start';
DROP TABLE recorded_deliveries,session_transitions,execution_jobs,session_inbox,session_steps,conversation_owners;
UPDATE automation_sessions SET status='failed' WHERE status IN ('stopped','cancelled');
ALTER TABLE automation_sessions DROP CONSTRAINT automation_sessions_status_check;
ALTER TABLE automation_sessions ADD CONSTRAINT automation_sessions_status_check CHECK (status IN ('pending','running','waiting','completed','failed'));
ALTER TABLE automation_sessions DROP COLUMN current_node_id,DROP COLUMN context,DROP COLUMN origin,DROP COLUMN version,DROP COLUMN finish_reason,DROP COLUMN updated_at;
ALTER TABLE automation_versions DROP COLUMN execution_plan;
