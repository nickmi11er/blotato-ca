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

Vitest covers trigger matching, draft validation/storage, and React form interactions using Testing Library and jsdom. Native dialog focus and browser layout require browser checks; jsdom only supplies a minimal dialog stub for interaction tests.

Tooling audit (2026-11-04): npm reports 11 related moderate findings from the ESLint dependency chain, originating in [minimatch GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249). The issue concerns resource exhaustion from attacker-controlled glob patterns. These packages are development dependencies and are not copied into the Nginx runtime image. The suggested `npm audit fix --force` downgrades core tooling to incompatible versions; update the dependency chain when compatible upstream releases become available instead of applying that downgrade.

## Structure

- `src/app`: application composition, workspace layout, and responsive layout overrides.
- `src/feature/automation`: builder components, trigger model/validation, draft state hook, storage adapter, demo fixtures, and feature styles.
- `src/shared/ui`: reusable UI primitives such as Modal, TagInput, Icon, and Section.
- `src/shared/styles`: fonts/tokens, global defaults, and feedback styling.
- Tests live next to the behavior they exercise. `src/test/setup.ts` contains test environment setup only.

The application passes accounts and posts into the feature as a catalog. Account identity is independent of its platform; existing fixture IDs are retained for compatibility with saved demo drafts. The model and storage adapter do not import fixtures. Unknown scope references remain visible and block saving until repaired; they never silently broaden a trigger. Message validation is shared by the message editor and draft persistence. Exact matching offers alternative full comments, while contradictory restored exact/all rules are reported by validation.

The preview owns its sample comment and conversation state. The draft hook owns editing and persistence, and derives unsaved changes by comparing the current draft to its saved snapshot. Storage remains browser-local and versioned.

The compact editor links back to `#automations`, a list of the current local automation. `#builder` opens its editor. Navigation uses hash history and keeps the draft mounted in the application, so moving between these screens preserves unsaved edits. This is still a single-automation prototype, not a backend-backed list or multi-automation CRUD interface.

To build and run the frontend in Docker, execute from the repository root:

```sh
docker compose up -d --build --wait frontend
```

Open `http://localhost:3000`. The Dockerfile builds the app with Node.js and serves the generated static files with Nginx. The frontend can run independently of the backend. Stop it with `docker compose stop frontend`.

The interface includes social account and post selection, exact/whole-word/substring matching, any/all keywords, exclusions, case sensitivity, repeat policies, editable messages, and an interactive email-capture conversation. Intent matching is shown as a future option.

Accounts and posts are fixtures. Drafts are saved only in browser local storage; no backend endpoints or social platform APIs are called. The tester runs the TypeScript evaluator locally. Production integration should use the backend evaluator for both preview and real events.

The visual reference supplies the dark plum palette, pink/violet accents, Geist fonts, and Blotato icon. This is an interview prototype, not an official Blotato application.
