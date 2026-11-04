-- name: ReserveParticipant :exec
INSERT INTO admission_locks (tenant_id, automation_id, account_id, participant_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING;
-- name: LockParticipant :one
SELECT participant_id FROM admission_locks WHERE tenant_id = $1 AND automation_id = $2 AND account_id = $3 AND participant_id = $4 FOR UPDATE;
-- name: PriorSessions :one
SELECT EXISTS (SELECT 1 FROM automation_sessions s WHERE s.tenant_id = $1 AND s.automation_id = $2 AND s.account_id = $3 AND s.participant_id = $4) AS started,
EXISTS (SELECT 1 FROM automation_sessions s WHERE s.tenant_id = $1 AND s.automation_id = $2 AND s.account_id = $3 AND s.participant_id = $4 AND status IN ('pending', 'running', 'waiting')) AS active;
-- name: CreateSession :exec
INSERT INTO automation_sessions (id, tenant_id, automation_id, revision, account_id, participant_id, origin) VALUES ($1, $2, $3, $4, $5, $6, $7);
-- name: RecordDecision :exec
INSERT INTO comment_decisions (platform, account_id, post_id, comment_id, event_type, outcomes) VALUES ($1, $2, $3, $4, $5, $6);
