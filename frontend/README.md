# Automation builder preview

React + TypeScript + Vite prototype inspired by Blotato's public website. Use Node.js 24 (`.nvmrc`), matching the Docker build environment. TypeScript 6 is pinned to the version range supported by typescript-eslint.

```sh
npm ci
npm run dev
```

Open the URL printed by Vite (normally `http://127.0.0.1:5173`). Run `npm run build` for a production build.

```sh
npm run check        # TypeScript (including tests), ESLint/React Hooks, Vitest
npm run format:check # Prettier
```

Vitest covers trigger matching, workflow decoding and migration, data dependencies, waiting-session simulation, templates, persistence, and React interactions using Testing Library and jsdom. Native dialog focus and browser layout require browser checks; jsdom only supplies a minimal dialog stub for interaction tests.

Tooling audit (2026-11-04): npm reports 11 related moderate findings from the ESLint dependency chain, originating in [minimatch GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249). The issue concerns resource exhaustion from attacker-controlled glob patterns. These packages are development dependencies and are not copied into the Nginx runtime image. The suggested `npm audit fix --force` downgrades core tooling to incompatible versions; update the dependency chain when compatible upstream releases become available instead of applying that downgrade.

## Structure

- `src/app`: application composition, workspace layout, and responsive layout overrides.
- `src/feature/automation`: workflow canvas and inspectors, trigger rules, typed step model, validation, pure simulation transitions, draft state, storage adapter, fixtures, and styles.
- `src/feature/email-templates`: reusable template collection, editor with live preview, validation, and local persistence.
- `src/shared/ui`: reusable UI primitives such as Modal, TagInput, Icon, and Section.
- `src/shared/styles`: fonts/tokens, global defaults, and feedback styling.
- Tests live next to the behavior they exercise. `src/test/setup.ts` contains test environment setup only.

The application passes accounts and posts into the feature as a catalog. Account identity is independent of its platform; existing fixture IDs are retained for compatibility with saved demo drafts. The model and storage adapter do not import fixtures. Unknown scope references remain visible and block testing until repaired; they never silently broaden a trigger. Incomplete drafts can be saved. Persistence validates structural readability; the test runner additionally validates execution readiness. Exact matching offers alternative full comments, while contradictory restored exact/all rules are reported by validation.

The test panel owns its sample comment and reply; the builder shares simulation state with the canvas to highlight completed steps, active waits, and outcomes. The automation collection hook owns editing and persistence, and derives unsaved changes by comparing each draft to its saved snapshot. Storage remains browser-local and versioned.

The compact editor links back to `#automations`, a list of local automations. New automation creates and saves an empty draft with a unique ID and opens the builder. Accounts, keywords, and actions start empty. Up to 100 automations can be stored on this device. `#builder` opens the selected automation, and `#templates` opens the email template collection. Selection is held in memory; reloading opens the last created or saved automation. These are page routes, not shareable per-automation URLs. Switching screens or automations preserves all unsaved edits in memory; saving or creating one automation does not implicitly save changes in another. Reloading only restores saved drafts. Template edits are saved separately with Save template. This remains a browser-local prototype without backend persistence or publishing. Each card has a menu for deleting its automation after confirmation. Deletion is persisted before updating the list, preserves other unsaved drafts in memory, and does not delete shared email templates. Removing the last automation leaves an empty collection; reloading never reimports the legacy draft into that collection.

To build and run the frontend in Docker, execute from the repository root:

```sh
docker compose up -d --build --wait frontend
```

Open `http://localhost:3000`. The Dockerfile builds the app with Node.js and serves the generated static files with Nginx. The frontend can run independently of the backend. Stop it with `docker compose stop frontend`.

The interface includes account/post selection, exact/whole-word/substring matching, any/all keywords, exclusions, case sensitivity, repeat policies, configurable actions, parallel groups, reply capture, delays, scheduled dates, and reusable email templates. Intent matching is shown as a future option.

Accounts and posts are fixtures. Drafts are saved only in browser local storage; no backend endpoints or social platform APIs are called. The tester runs the TypeScript evaluator locally. Production integration should use the backend evaluator for both preview and real events.

The visual reference supplies the dark plum palette, pink/violet accents, Geist fonts, and Blotato icon. This is an interview prototype, not an official Blotato application.

## Workflow model and interaction

This version uses an ordered, structured workflow rather than a free-form graph. A user adds a step at any connection, selects it to edit its settings, and uses the card menu to move it up/down within its path. Duplication generates new IDs recursively; deleting, adding, duplicating, or moving a step offers a one-operation Undo. Message actions and email selections start empty; examples are opt-in. Parallel groups show all actions by default and can be collapsed manually. Card menus close on outside click/tap, focus leaving the menu, or Escape. The initial inspector opens the trigger. This covers the requested automation while avoiding disconnected nodes, accidental cycles, and arbitrary joins. The scope is intentionally bounded: up to 40 steps per path, 8 immediate actions per parallel group, 3 levels of timeout branches, and 160 nodes overall. A timeout may follow a separate chain that ends independently, without rejoining the success continuation. General conditional branches, nested parallel workflows, and arbitrary loops are not implemented.

The default sequence is: matching comment → public reply and source-account DM in parallel → wait for an email → wait one hour → send the pricing email template. The timer is optional and can be removed. A DM action can also deliver a link directly, preserving the original take-home scenario.

`Draft.schemaVersion = 2` stores trigger scope/rules and a discriminated union of steps with stable IDs:

| Kind         | Configuration                                                     | Completion                                                   |
| ------------ | ----------------------------------------------------------------- | ------------------------------------------------------------ |
| `reply`      | Message text                                                      | Public reply to the original comment                         |
| `dm`         | Message text                                                      | DM to the same person through the source social account      |
| `email`      | Template ID                                                       | Email to previously captured `contact.email`                 |
| `parallel`   | Immediate child actions                                           | All children succeed; order is unspecified                   |
| `wait_reply` | Expected email/text, timeout, invalid-reply policy, attempt limit | A valid new DM resumes the next step                         |
| `delay`      | Minutes                                                           | Resume after the duration, measured from entering this step  |
| `schedule`   | UTC instant or next wall-clock time, IANA timezone                | Resume at the instant; continue immediately if it has passed |

Email capture is a purpose-built waiting step with visible outcomes: success saves `contact.email`; an invalid reply either ends the run or sends a retry DM; a timeout ends the run, optionally after a final DM, or takes a separately configured path. Clicking an outcome opens its settings. Email availability is checked independently on each path: a timed-out wait does not supply an email. The invalid-reply limit includes the final rejected message: a limit of 3 permits 2 retry DMs, then stops on the third rejection. Invalid replies do not extend the original timeout. The parser accepts one unambiguous ASCII email inside natural text; it checks format, not ownership or deliverability. Multiple distinct addresses need clarification. Any-text capture does not create an email variable.

Testing validation blocks missing templates, email actions before successful capture, invalid time/duration limits, unsupported DM accounts, and ambiguous reentry. A flow waiting for DMs cannot start on every matching comment while an earlier run is active. The workflow simulator exercises these configured paths without sending anything. The test panel shows captured contact data and keeps its event journal scrolled to the latest event. Timers and timeouts are advanced explicitly by the tester; no real scheduler is running in the browser.

Email templates store ID, name, subject, and plain-text body with `{{first_name}}` / `{{email}}` variables. React renders the preview as text, without HTML injection. Templates are reusable within the local draft; changing one updates every draft step using its ID. Deletion is blocked while any in-memory or saved automation references the template. A production published workflow should pin an immutable template revision.

Storage uses `blotato.automations.v1` and `blotato.email-templates.v1`. When no collection exists, the original `blotato.trigger-draft.v2` draft is imported as its first entry without overwriting the legacy key. An unreadable collection blocks writes so it cannot be overwritten accidentally. Existing v1 message drafts migrate to equivalent reply/DM → email capture → DM link steps, preserving the original texts and destination. Migration does not overwrite v1 storage. Unreadable v2 steps become a visibly incomplete chain requiring repair, never substitute executable default actions. Storage writes check structure while allowing incomplete configuration; failures leave in-memory edits intact. Cross-tab editing is not synchronized: the last successful local save wins.

Scheduled steps support either a concrete instant or the next occurrence of a wall-clock time in a recorded IANA timezone. The latter runs once per session; it skips nonexistent daylight-saving times and chooses the next real occurrence during repeated hours. Duration inputs use minutes, hours, or days. The simulator maintains a virtual clock across successive timers and timeout branches.

## Backend execution contract (proposed, not implemented)

The current backend receives and deduplicates comments only. The editor, template store, matching preview, and simulator remain browser-local. This UI change does not add workflow persistence endpoints, a social/email dispatcher, or a durable scheduler.

For integration, publish immutable workflow versions containing validated steps and pinned template revisions. Keep draft edits separate from running versions. A run needs its workflow version, original comment ID, tenant/account/person/conversation IDs, cursor, captured variables, status, and revision. Step executions track state and attempts independently, especially parallel children. A waiting record stores expected event type, correlation key, deadline or wake time, invalid count, and a consumption cursor; an outbox stores outgoing commands. Account capabilities must be validated again on the server.

Transactions advance a run and insert outgoing commands together. A worker dispatches commands with stable idempotency keys such as run/step/action ID and records results; retry only failed children of a parallel group. External delivery is not magically exactly-once: use provider idempotency when available, reconcile ambiguous results, and expose terminal failures. Apply per-account rate limits and bounded retries. A parallel group is a completion barrier, not an atomic social-platform transaction; one public reply can succeed while its sibling DM fails.

Waiting releases workers. Durable rows and indexed due times survive restarts; leased workers claim due timers and compare-and-swap the run revision before advancing. A reply and a timeout must contend for the same waiting transition, so only one wins. Specify the boundary consistently: accept an unconsumed inbox event durably received before the deadline; the timeout transaction checks that inbox before expiring the run. Deduplicate incoming DMs as well as comments. Never reuse one DM for successive waiting steps or let an older message satisfy a new prompt.

Avoid the fast-reply race: persist the outbound prompt and the reply correlation boundary before dispatch, and retain incoming DMs in a durable inbox. A reply arriving while the public-reply/DM parallel group is still completing can then be consumed when the waiting step becomes active. Correlate by tenant, social account, person, and conversation; do not use display names or email as identity. Enforce the one-active-run rule transactionally. Across different automations, competing reply captures need a conversation-level reservation or explicit routing policy before publication; the local simulator does not arbitrate between concurrently running automations.

## Contribution note

This iteration was implemented with Codex assistance: workflow UI, step and template models, local simulator, migration/validation, tests, styles, and this explanation were AI-assisted. The product requirements and requested interaction changes came from the repository owner. The final interview submission should separately describe the owner's own design, implementation, and review work; this note does not claim work on their behalf.
