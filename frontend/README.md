# Automation builder

React + TypeScript + Vite. The editor loads automations, account/post metadata, and email templates from the Go backend. PostgreSQL is the source of truth; browser storage is no longer used by the application.

## Run

From the repository root:

```sh
docker compose up -d --build --wait
```

Open `http://localhost:3000`. On first startup, `demo-init` seeds the `northstar` workspace with three email templates, a ready-to-publish “Pricing → email capture” draft, and sample account/post metadata. The draft matches `pricing`, replies publicly and asks for an email in parallel, waits for a valid email, then sends the pricing template. It is not published automatically. Initialization runs once per database; subsequent starts preserve edits and deletions. Demo data stays outside application migrations.

For hot reload, keep the backend running, then use Node.js 24 (`.nvmrc`):

```sh
npm ci
cp .env.example .env
npm run dev
```

Vite proxies `/api` to the backend. Docker uses Nginx for the same boundary. Both fix the workspace on the server (`northstar` locally), allow only editor routes, and inject the internal bearer credential server-side. The credential is never a `VITE_` variable or part of the browser bundle. This loopback-only deployment has no user login; a public deployment must derive the workspace from an authenticated gateway session instead of this fixed local configuration.

`npm run build` creates static assets. `vite preview` does not provide the API proxy; use Compose to test the complete production build.

## Editing and publishing

- Create, save, list, and delete automations through the API. The selected automation is identified by the URL query parameter, so reloads and returning from templates preserve the editor; unavailable IDs return to the list. Incomplete drafts can be saved; publication validates trigger rules, account capabilities, the workflow, and its dependencies.
- Saving a published flow changes only its draft. **Publish changes** creates an immutable version for new sessions. **Pause** disables new admission; existing sessions continue. Deleting an automation also preserves existing sessions and published history.
- Draft saves, publication, and deletion send `If-Match` with the draft version. A stale editor receives a conflict instead of overwriting another tab's edits. Network errors keep the editor and its unsaved changes intact; edits typed during an in-flight save remain dirty.
- Templates have independent revisions. Publication resolves referenced templates in the same workspace transaction and pins their contents in the execution plan. Editing a template never changes existing publications or sessions. Templates referenced by saved drafts cannot be deleted.
- Account identity is independent of the platform. Post selections use `(account_id, post_id)` so equal provider IDs on different accounts remain distinct. Mixed per-account scopes and explicit DM-question references survive editing.
- **Test flow** is a local simulator with a virtual clock. It never publishes events or sends messages. Published flows execute on the backend; the checked-in delivery configuration records outgoing requests locally. See [execution and delivery](../docs/architecture/003-session-execution-and-dm-routing.md).

Old prototype localStorage keys are left untouched and are not automatically imported: their demo account IDs may not identify connected accounts. The legacy storage decoders remain isolated migration utilities, covered by tests, and are not used in the runtime application.

## Structure and verification

`src/app` loads and composes the workspace. `feature/automation/api` maps API documents and compound post identities; `model/useAutomations` owns editing state and asynchronous mutations. Workflow models, validation, canvas, and simulation stay within that feature. `feature/email-templates` owns its editor and API-backed state; `shared/api` handles HTTP errors and conditional-write headers.

```sh
npm run check        # TypeScript, ESLint/React Hooks, Vitest
npm run build
npm run format:check
```

Tests cover API persistence and remounting, incomplete drafts, failed requests, conflicting writes, edits during saves, publication/pause, deletion, template creation, workflow simulation, and trigger matching. Backend integration tests verify workspace isolation, draft versions, catalog identities, immutable template snapshots, and actual execution. Native dialog/layout checks use a browser.

## Contribution note

The API integration, UI state handling, tests, and documentation in this iteration were developed with Codex assistance. Product requirements came from the repository owner. The final interview submission should separately describe the owner's design, implementation, and review work.
