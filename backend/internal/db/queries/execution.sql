-- name: GetSession :one
SELECT * FROM automation_sessions WHERE id=$1;
-- name: LockSession :one
SELECT * FROM automation_sessions WHERE id=$1 FOR UPDATE;
-- name: GetPlan :one
SELECT document,execution_plan FROM automation_versions WHERE tenant_id=$1 AND automation_id=$2 AND revision=$3;
-- name: EnsureConversation :exec
INSERT INTO conversation_owners(tenant_id,account_id,participant_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;
-- name: LockConversation :one
SELECT * FROM conversation_owners WHERE account_id=$1 AND participant_id=$2 FOR UPDATE;
-- name: Route :one
SELECT c.session_id,c.epoch,c.active,a.platform FROM conversation_owners c JOIN connected_accounts a ON a.id=c.account_id WHERE c.account_id=$1 AND c.participant_id=$2;
-- name: ReserveConversation :one
UPDATE conversation_owners SET session_id=$3,epoch=epoch+1,active=false WHERE account_id=$1 AND participant_id=$2 AND session_id IS NULL RETURNING epoch;
-- name: ActivateConversation :execrows
UPDATE conversation_owners SET active=true WHERE account_id=$1 AND participant_id=$2 AND session_id=$3 AND epoch=$4;
-- name: ReleaseConversation :exec
UPDATE conversation_owners SET session_id=NULL,active=false WHERE account_id=$1 AND participant_id=$2 AND session_id=$3;
-- name: NextSequence :one
UPDATE conversation_owners SET next_sequence=next_sequence+1 WHERE account_id=$1 AND participant_id=$2 RETURNING next_sequence;
-- name: InsertDM :one
INSERT INTO session_inbox(account_id,participant_id,conversation_id,provider_message_id,session_id,conversation_epoch,sequence,payload)
VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING RETURNING id;
-- name: NextReply :one
SELECT * FROM session_inbox WHERE session_id=$1 AND consumed_at IS NULL AND sequence>$2 AND accepted_at<$3 ORDER BY sequence LIMIT 1;
-- name: ConsumeReply :exec
UPDATE session_inbox SET consumed_at=clock_timestamp() WHERE id=$1;
-- name: InitStep :exec
INSERT INTO session_steps(session_id,node_id,parent_node_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;
-- name: ListSteps :many
SELECT * FROM session_steps WHERE session_id=$1 ORDER BY node_id;
-- name: SaveStep :exec
UPDATE session_steps SET status=$3,wait_kind=$4,listen_from_sequence=$5,deadline_at=$6,invalid_attempts=$7,output=$8 WHERE session_id=$1 AND node_id=$2;
-- name: UpdateSession :exec
UPDATE automation_sessions SET status=$2,current_node_id=$3,context=$4,version=version+1,finish_reason=$5,updated_at=clock_timestamp() WHERE id=$1;
-- name: Transition :exec
INSERT INTO session_transitions(session_id,version,node_id,event_id,from_state,to_state,reason) VALUES($1,$2,$3,$4,$5,$6,$7);
-- name: Enqueue :exec
INSERT INTO execution_jobs(id,session_id,node_id,logical_key,kind,run_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(session_id,logical_key) DO NOTHING;
-- name: ClaimJob :one
WITH candidate AS (SELECT id FROM execution_jobs WHERE status='ready' AND run_at<=statement_timestamp() ORDER BY run_at,id LIMIT 1 FOR UPDATE SKIP LOCKED)
UPDATE execution_jobs j SET status='leased',lease_until=clock_timestamp()+sqlc.arg(lease_ms)::bigint*interval '1 millisecond',lease_token=lease_token+1,attempt=attempt+1
FROM candidate c WHERE j.id=c.id RETURNING j.*;
-- name: RecoverJobs :execrows
UPDATE execution_jobs SET status=CASE WHEN kind='deliver' AND NOT sqlc.arg(safe_delivery)::boolean THEN 'unknown' ELSE 'ready' END,lease_token=lease_token+1,lease_until=NULL WHERE status='leased' AND lease_until<=statement_timestamp();
-- name: LockJob :one
SELECT * FROM execution_jobs WHERE id=$1 FOR UPDATE;
-- name: FinishJob :execrows
UPDATE execution_jobs SET status=$3,result=$4,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND status='leased' AND lease_until>clock_timestamp();
-- name: RetryJob :execrows
UPDATE execution_jobs SET status='ready',run_at=clock_timestamp()+sqlc.arg(delay_ms)::bigint*interval '1 millisecond',lease_until=NULL,result=$3 WHERE id=$1 AND lease_token=$2 AND status='leased' AND lease_until>clock_timestamp();
-- name: CancelJobs :exec
UPDATE execution_jobs SET status='cancelled',lease_token=lease_token+1,lease_until=NULL WHERE session_id=$1 AND status IN ('ready','leased','unknown') AND id<>$2;
-- name: CancelWaitJobs :exec
UPDATE execution_jobs SET status='cancelled',lease_token=lease_token+1,lease_until=NULL WHERE session_id=$1 AND node_id=$2 AND status='ready' AND id<>$3;
-- name: ListJobs :many
SELECT * FROM execution_jobs WHERE session_id=$1 ORDER BY run_at,id;
-- name: ListTransitions :many
SELECT * FROM session_transitions WHERE session_id=$1 ORDER BY version;
-- name: RecordDelivery :exec
INSERT INTO recorded_deliveries(idempotency_key,payload) VALUES($1,$2) ON CONFLICT DO NOTHING;
-- name: RecordedDelivery :one
SELECT payload FROM recorded_deliveries WHERE idempotency_key=$1;
-- name: ClaimUnknownJob :one
UPDATE execution_jobs SET status='leased',lease_token=lease_token+1,lease_until=clock_timestamp()+interval '30 seconds' WHERE id=$1 AND session_id=$2 AND status='unknown' RETURNING *;
-- name: DatabaseTime :one
SELECT clock_timestamp()::timestamptz;
