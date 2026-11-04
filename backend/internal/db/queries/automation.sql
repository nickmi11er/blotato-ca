-- name: EnsureWorkspace :exec
INSERT INTO workspaces (id) VALUES ($1) ON CONFLICT DO NOTHING;
-- name: LockWorkspace :one
SELECT id FROM workspaces WHERE id = $1 FOR UPDATE;
-- name: RegisterAccount :execrows
INSERT INTO connected_accounts (id, tenant_id, platform,name,handle,can_send_dm) VALUES ($1, $2, $3,$3,$1,$3<>'youtube') ON CONFLICT DO NOTHING;
-- name: GetAccount :one
SELECT * FROM connected_accounts WHERE id = $1;
-- name: CountAutomations :one
SELECT count(*) FROM automations WHERE tenant_id = $1;
-- name: GetAutomation :one
SELECT * FROM automations WHERE tenant_id = $1 AND id = $2;
-- name: SaveDraft :exec
INSERT INTO automations (tenant_id, id, draft) VALUES ($1, $2, $3)
ON CONFLICT (tenant_id, id) DO UPDATE SET draft = EXCLUDED.draft, draft_version=automations.draft_version+1, updated_at=clock_timestamp();
-- name: NextRevision :one
SELECT (COALESCE(max(revision), 0) + 1)::bigint FROM automation_versions WHERE tenant_id = $1 AND automation_id = $2;
-- name: AddVersion :exec
INSERT INTO automation_versions (tenant_id, automation_id, revision, document, execution_plan) VALUES ($1, $2, $3, $4, $5);
-- name: SetPublished :exec
UPDATE automations SET revision = $3, enabled = true, published_draft_version=draft_version WHERE tenant_id = $1 AND id = $2;
-- name: DisableAutomation :exec
UPDATE automations SET enabled = false WHERE tenant_id = $1 AND id = $2;
-- name: DeleteAutomation :exec
DELETE FROM automations WHERE tenant_id = $1 AND id = $2;
-- name: BoundAccounts :many
SELECT account_id FROM trigger_bindings WHERE tenant_id = $1 AND automation_id = $2 ORDER BY account_id;
-- name: ClearBindings :exec
DELETE FROM trigger_bindings WHERE tenant_id = $1 AND automation_id = $2;
-- name: BindAccount :exec
INSERT INTO trigger_bindings (tenant_id, account_id, automation_id, revision, post_scope) VALUES ($1, $2, $3, $4, $5);
-- name: BindPost :exec
INSERT INTO trigger_posts (tenant_id, account_id, post_id, automation_id) VALUES ($1, $2, $3, $4);
-- name: AdvanceGeneration :one
INSERT INTO account_rule_sets (account_id, generation) VALUES ($1, 1)
ON CONFLICT (account_id) DO UPDATE SET generation = account_rule_sets.generation + 1 RETURNING generation;
-- name: AddRuleChange :exec
INSERT INTO rule_changes (account_id, generation) VALUES ($1, $2);
-- name: PendingRuleChanges :many
SELECT id, account_id, generation FROM rule_changes ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED;
-- name: DeleteRuleChanges :exec
DELETE FROM rule_changes WHERE id = ANY($1::bigint[]);
-- name: AccountSnapshot :one
SELECT a.tenant_id, a.platform, COALESCE(r.generation, 0)::bigint AS generation,
    EXISTS (SELECT 1 FROM trigger_bindings b WHERE b.account_id = a.id AND b.post_scope = 'selected') AS has_selected
FROM connected_accounts a LEFT JOIN account_rule_sets r ON r.account_id = a.id WHERE a.id = $1;
-- name: AccountRules :many
SELECT b.automation_id, b.revision, v.document
FROM trigger_bindings b JOIN automation_versions v USING (tenant_id, automation_id, revision)
WHERE b.account_id = $1 AND b.post_scope = 'all' ORDER BY b.automation_id;
-- name: PostRules :many
SELECT b.automation_id, b.revision, v.document
FROM trigger_posts p JOIN trigger_bindings b USING (tenant_id, account_id, automation_id)
JOIN automation_versions v USING (tenant_id, automation_id, revision)
WHERE p.account_id = $1 AND p.post_id = $2 ORDER BY b.automation_id;
-- name: Generations :many
SELECT k.account_id::text AS account_id, COALESCE(r.generation, 0)::bigint AS generation
FROM unnest($1::text[]) AS k(account_id) LEFT JOIN account_rule_sets r ON r.account_id = k.account_id;
-- name: ListAutomations :many
SELECT * FROM automations WHERE tenant_id=$1 ORDER BY updated_at DESC,id;
-- name: ListAccounts :many
SELECT * FROM connected_accounts WHERE tenant_id=$1 ORDER BY platform,id;
-- name: UpdateAccountDetails :exec
UPDATE connected_accounts SET name=$3,handle=$4,can_send_dm=$5 WHERE tenant_id=$1 AND id=$2;
-- name: ListPosts :many
SELECT * FROM social_posts WHERE tenant_id=$1 ORDER BY published_at DESC,account_id,id;
-- name: SavePost :exec
INSERT INTO social_posts(tenant_id,account_id,id,title,published_at) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(account_id,id) DO UPDATE SET title=EXCLUDED.title,published_at=EXCLUDED.published_at;
-- name: ListTemplates :many
SELECT * FROM email_templates WHERE tenant_id=$1 ORDER BY name,id;
-- name: GetTemplate :one
SELECT * FROM email_templates WHERE tenant_id=$1 AND id=$2;
-- name: CountTemplates :one
SELECT count(*) FROM email_templates WHERE tenant_id=$1;
-- name: SaveTemplate :exec
INSERT INTO email_templates(tenant_id,id,name,subject,body) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(tenant_id,id) DO UPDATE SET name=EXCLUDED.name,subject=EXCLUDED.subject,body=EXCLUDED.body,revision=email_templates.revision+1;
-- name: DeleteTemplate :exec
DELETE FROM email_templates WHERE tenant_id=$1 AND id=$2;
