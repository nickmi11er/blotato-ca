#!/bin/sh
set -eu

# Backend owns migrations; wait for its editor schema before inserting fixtures.
attempt=0
until [ "$(psql -Atqc "SELECT to_regclass('email_templates') IS NOT NULL AND EXISTS (SELECT 1 FROM goose_db_version WHERE version_id=4 AND is_applied)" 2>/dev/null || true)" = "t" ]; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
        echo 'Backend migrations were not ready within 60 seconds.' >&2
        exit 1
    fi
    sleep 1
done
psql -v ON_ERROR_STOP=1 -f /seed/seed-demo.sql
