-- Local Compose fixtures, deliberately separate from application migrations.
-- One transaction and a persistent marker preserve edits and deletions on restart.
BEGIN;
SELECT pg_advisory_xact_lock(483170251);
CREATE TABLE IF NOT EXISTS demo_seed_state (
    name text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);
DO $seed$
BEGIN
    IF EXISTS (SELECT 1 FROM demo_seed_state WHERE name='northstar-v1') THEN
        RETURN;
    END IF;

    INSERT INTO workspaces(id) VALUES ('northstar') ON CONFLICT DO NOTHING;
    PERFORM id FROM workspaces WHERE id='northstar' FOR UPDATE;
    INSERT INTO connected_accounts(id,tenant_id,platform,name,handle,can_send_dm) VALUES
        ('northstar_instagram','northstar','instagram','Instagram','@northstar.studio',true),
        ('northstar_facebook','northstar','facebook','Facebook','Northstar Studio',true),
        ('northstar_youtube','northstar','youtube','YouTube','@northstarstudio',false)
    ON CONFLICT DO NOTHING;
    INSERT INTO social_posts(tenant_id,account_id,id,title,published_at) VALUES
        ('northstar','northstar_instagram','ig_1','A little idea. A big beginning.','2026-10-30T10:00:00Z'),
        ('northstar','northstar_instagram','ig_2','Your next chapter starts here.','2026-10-28T10:00:00Z'),
        ('northstar','northstar_facebook','fb_1','Meet your new creative toolkit.','2026-10-29T10:00:00Z'),
        ('northstar','northstar_youtube','yt_1','A better way to create.','2026-10-27T10:00:00Z')
    ON CONFLICT DO NOTHING;

    INSERT INTO email_templates(tenant_id,id,name,subject,body) VALUES
        ('northstar','pricing-intro','Pricing & next steps','Your pricing link',
         E'Hi {{first_name}},\n\nThanks for your interest! Find the right plan for you:\nhttps://example.com/pricing\n\nHave a question? Just reply to this email.\n\nThe Northstar team'),
        ('northstar','welcome-note','A warm welcome','Welcome aboard, {{first_name}}!',
         E'Hi {{first_name}},\n\nWe are glad you are here. Explore ideas and resources to get started:\nhttps://example.com/welcome\n\nThe Northstar team'),
        ('northstar','friendly-follow-up','A friendly follow-up','Any questions about your next step?',
         E'Hi {{first_name}},\n\nHave you had a chance to explore our plans? Here is your link again:\nhttps://example.com/pricing\n\nReply whenever you are ready. We are happy to help.\n\nThe Northstar team')
    ON CONFLICT DO NOTHING;

    -- Ready to publish from the UI. No live trigger bindings or sends on startup.
    INSERT INTO automations(tenant_id,id,draft) VALUES (
        'northstar','pricing-email-capture',
        $document${
          "name": "Pricing → email capture",
          "trigger": {
            "schema_version": 1,
            "scopes": [
              {"account_id": "northstar_instagram", "posts": {"mode": "all"}},
              {"account_id": "northstar_facebook", "posts": {"mode": "all"}}
            ],
            "comment": {"mode": "keywords", "operator": "whole_word", "logic": "any", "keywords": ["pricing"], "exclusions": [], "case_sensitive": false},
            "repeat": {"mode": "skip_active"}
          },
          "workflow": {
            "schema_version": 1,
            "steps": [
              {"id": "welcome", "kind": "parallel", "actions": [
                {"id": "public-reply", "kind": "reply", "text": "Sent you a DM!"},
                {"id": "ask-email", "kind": "dm", "text": "Thanks for your interest! What is the best email address to send our pricing to?"}
              ]},
              {"id": "capture-email", "kind": "wait_reply", "prompt_node_id": "ask-email", "expected": "email", "timeoutHours": 24, "invalidBehavior": "retry", "retryMessage": "Please send one email address, for example you@example.com.", "maxAttempts": 3, "timeoutBehavior": "stop", "timeoutMessage": ""},
              {"id": "send-pricing", "kind": "email", "templateId": "pricing-intro"}
            ]
          }
        }$document$::jsonb
    ) ON CONFLICT DO NOTHING;
    INSERT INTO demo_seed_state(name) VALUES ('northstar-v1');
END
$seed$;
COMMIT;
