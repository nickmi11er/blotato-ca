# README visuals

- `interface.png` — element screenshot of only the seeded pricing flow canvas (`.wf-canvas`), captured at a 1440 px viewport with 2× pixel density. Navigation, the automation toolbar and the settings inspector are outside the capture. The capture does not publish or edit the automation.
- `hero.html` — presentation frame for that capture. Open at a 1600 px viewport, wait for the local Geist font to load, and take a full-page screenshot to regenerate `flow-preview.png`.
- `architecture.svg` — self-contained vector illustration of the comment-to-DM execution path, read from top to bottom. Numbered stages are internal Go modules; side panels show PostgreSQL, incoming replies and integration boundaries. The bottom strip shows publication and cache invalidation. All labels remain editable text.
- `architecture.mmd` — Mermaid source describing the same flow, including explicit retry and delivery edges. Keep it and the SVG aligned when deployment boundaries or data paths change.

These assets represent the current implementation. Social ingestion and messaging delivery are existing integration boundaries; the checked-in configuration records outbound requests locally.
