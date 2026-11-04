import { StrictMode } from "react";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test, vi } from "vitest";
import App from "./App";
import { defaultDraft } from "../feature/automation/model/defaultDraft";
import { accounts, posts } from "../feature/automation/fixtures";
import {
  defaultTemplates,
  type EmailTemplate,
} from "../feature/email-templates/model/templates";
import {
  encodeDraft,
  type AutomationResponse,
} from "../feature/automation/api/automations";

let records: AutomationResponse[];
let templates: EmailTemplate[];
let calls: { path: string; method: string; options: RequestInit }[];
let intercept:
  | ((
      path: string,
      options: RequestInit,
    ) => Promise<Response> | Response | undefined)
  | undefined;
const json = (value: unknown) =>
  new Response(JSON.stringify(value), {
    headers: { "Content-Type": "application/json" },
  });
const empty = () => new Response(null, { status: 204 });
function record(id = "first", name = defaultDraft.name): AutomationResponse {
  return {
    id,
    enabled: false,
    revision: 0,
    draft_version: 1,
    published_draft_version: 0,
    updated_at: new Date().toISOString(),
    draft: encodeDraft({ ...structuredClone(defaultDraft), name }),
  };
}
beforeEach(() => {
  records = [record()];
  templates = defaultTemplates.map((t) => ({ ...t, revision: 1 }));
  calls = [];
  intercept = undefined;
  window.history.replaceState(null, "", "/#builder");
  vi.spyOn(globalThis, "fetch").mockImplementation(
    async (input, options = {}) => {
      const path = String(input).replace("/api/", "");
      const method = options.method ?? "GET";
      calls.push({ path, method, options });
      const overridden = intercept?.(path, options);
      if (overridden) return overridden;
      if (path === "catalog")
        return json({
          accounts,
          posts: posts.map((p) => ({
            ...p,
            publishedAt: "2026-10-30T10:00:00Z",
          })),
        });
      if (path === "automations") return json(records);
      if (path === "templates") return json(templates);
      const [resource, id, action] = path.split("/");
      if (resource === "templates") {
        if (method === "PUT") {
          const body = JSON.parse(String(options.body)) as EmailTemplate;
          templates = templates
            .filter((t) => t.id !== id)
            .concat({ ...body, id, revision: (body.revision ?? 0) + 1 });
          return empty();
        }
        if (method === "DELETE") {
          templates = templates.filter((t) => t.id !== id);
          return empty();
        }
      }
      const index = records.findIndex((r) => r.id === id),
        entry = records[index];
      if (method === "GET")
        return entry ? json(entry) : new Response("Not found", { status: 404 });
      const expected = Number(
        new Headers(options.headers).get("If-Match")?.replaceAll('"', ""),
      );
      if (expected !== (entry?.draft_version ?? 0))
        return new Response(
          "this item changed in another window; reload before saving",
          { status: 409 },
        );
      if (method === "PUT") {
        const next = {
          ...(entry ?? record(id)),
          draft: JSON.parse(
            String(options.body),
          ) as AutomationResponse["draft"],
          draft_version: expected + 1,
        };
        if (index < 0) records.push(next);
        else records[index] = next;
      } else if (method === "DELETE")
        records = records.filter((r) => r.id !== id);
      else if (action === "publish") {
        entry.enabled = true;
        entry.revision++;
        entry.published_draft_version = entry.draft_version;
      } else if (action === "disable") entry.enabled = false;
      return empty();
    },
  );
});
async function boot() {
  const view = render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
  await screen.findByRole("button", { name: "Save draft" });
  return view;
}
async function rules(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
}

test("loads from the API, saves through blur, and restores server state after remount", async () => {
  const user = userEvent.setup();
  const view = await boot();
  await rules(user);
  await user.selectOptions(
    screen.getByLabelText("When there are multiple"),
    "all",
  );
  await user.selectOptions(screen.getByLabelText("Match type"), "exact");
  await user.type(screen.getByRole("textbox", { name: "Keywords" }), "guide");
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() =>
    expect(records[0].draft.trigger.comment.keywords).toEqual([
      "pricing",
      "guide",
    ]),
  );
  expect(records[0].draft.trigger.comment.logic).toBe("any");
  expect(localStorage.length).toBe(0);
  expect(
    calls.every((c) => !new Headers(c.options.headers).has("Authorization")),
  ).toBe(true);
  view.unmount();
  await boot();
  await rules(user);
  expect(screen.getByRole("button", { name: "Remove guide" })).toBeTruthy();
});

test("empty server state creates and persists an incomplete automation", async () => {
  records = [];
  window.history.replaceState(null, "", "#automations");
  const user = userEvent.setup();
  render(<App />);
  await user.click(
    await screen.findByRole("button", { name: "Create your first automation" }),
  );
  await screen.findByRole("heading", { name: "Untitled automation" });
  expect(records).toHaveLength(1);
  expect(records[0].draft.trigger.scopes).toEqual([]);
  expect(records[0].draft.workflow.steps).toEqual([]);
  expect(
    screen.getByRole<HTMLButtonElement>("button", { name: "Publish" }).disabled,
  ).toBe(true);
});

test("failed save retains edits and can be retried", async () => {
  const user = userEvent.setup();
  await boot();
  await rules(user);
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  intercept = (_, o) =>
    o.method === "PUT" ? new Response("down", { status: 503 }) : undefined;
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await screen.findByText(/server is unavailable/);
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  expect(records[0].draft.trigger.comment.operator).toBe("whole_word");
  intercept = undefined;
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await screen.findByText("Draft saved.");
  expect(records[0].draft.trigger.comment.operator).toBe("contains");
});

test("editing while saving does not mark newer edits as saved", async () => {
  const user = userEvent.setup();
  await boot();
  await rules(user);
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  let release!: (response: Response) => void;
  intercept = (_, o) =>
    o.method === "PUT"
      ? new Promise<Response>((resolve) => {
          release = resolve;
        })
      : undefined;
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await user.selectOptions(screen.getByLabelText("Match type"), "exact");
  release(empty());
  await screen.findByText("Draft saved.");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  expect(screen.getByLabelText<HTMLSelectElement>("Match type").value).toBe(
    "exact",
  );
});

test("publishing saves first, activates the flow, and pausing preserves the draft", async () => {
  const user = userEvent.setup();
  await boot();
  await rules(user);
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  await user.click(screen.getByRole("button", { name: "Publish" }));
  await screen.findByText("Published");
  expect(records[0].enabled).toBe(true);
  expect(records[0].published_draft_version).toBe(2);
  expect(
    calls.filter((c) => c.method !== "GET").map((c) => c.method + " " + c.path),
  ).toEqual(["PUT automations/first", "POST automations/first/publish"]);
  await user.click(screen.getByRole("button", { name: "Pause" }));
  await screen.findByText("Paused");
  expect(records[0].enabled).toBe(false);
});

test("a failed publication never displays a published state", async () => {
  const user = userEvent.setup();
  await boot();
  intercept = (p) =>
    p.endsWith("/publish")
      ? new Response("Choose a question to listen for", { status: 400 })
      : undefined;
  await user.click(screen.getByRole("button", { name: "Publish" }));
  await screen.findByText("Choose a question to listen for");
  expect(screen.queryByText("Published")).toBeNull();
  expect(screen.getByText("Draft")).toBeTruthy();
});

test("conflicts preserve local edits instead of overwriting a newer server draft", async () => {
  const user = userEvent.setup();
  await boot();
  await rules(user);
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  records[0].draft_version = 2;
  records[0].draft.name = "Other window";
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await screen.findByText(/changed in another window/);
  expect(records[0].draft.name).toBe("Other window");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
});

test("loading failure shows retry, never a misleading empty workspace", async () => {
  intercept = () => new Response("unavailable", { status: 503 });
  const user = userEvent.setup();
  render(<App />);
  await screen.findByRole("heading", { name: "Couldn’t load your workspace" });
  expect(screen.queryByText("Your next conversation starts here.")).toBeNull();
  intercept = undefined;
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await screen.findByRole("button", { name: "Save draft" });
});

test("deletion waits for the server and keeps the confirmation on failure", async () => {
  window.history.replaceState(null, "", "#automations");
  const user = userEvent.setup();
  render(<App />);
  await user.click(
    await screen.findByRole("button", {
      name: `Options for ${defaultDraft.name}`,
    }),
  );
  await user.click(screen.getByRole("button", { name: "Delete automation" }));
  intercept = (_, o) =>
    o.method === "DELETE"
      ? new Response("unavailable", { status: 503 })
      : undefined;
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Delete automation",
    }),
  );
  await screen.findByRole("alert");
  expect(records).toHaveLength(1);
  intercept = undefined;
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Delete automation",
    }),
  );
  await screen.findByRole("button", { name: "Create your first automation" });
  expect(records).toHaveLength(0);
});

test("deleting another automation preserves unsaved edits without saving them", async () => {
  records.push(record("second", "Second"));
  const user = userEvent.setup();
  await boot();
  await rules(user);
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  await user.click(screen.getByRole("link", { name: "Automations" }));
  await user.click(
    await screen.findByRole("button", { name: "Options for Second" }),
  );
  await user.click(screen.getByRole("button", { name: "Delete automation" }));
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Delete automation",
    }),
  );
  await waitFor(() => expect(records).toHaveLength(1));
  expect(records[0].draft.trigger.comment.operator).toBe("whole_word");
  await user.click(
    screen.getByRole("link", { name: `Edit ${defaultDraft.name}` }),
  );
  await rules(user);
  expect(screen.getByLabelText<HTMLSelectElement>("Match type").value).toBe(
    "contains",
  );
});

test("email templates persist through the API and can be selected by a step", async () => {
  const user = userEvent.setup();
  await boot();
  await user.click(screen.getByRole("link", { name: "Email templates" }));
  await user.click(await screen.findByRole("button", { name: "New template" }));
  await user.type(screen.getByLabelText("Template name"), "Follow-up");
  await user.type(screen.getByLabelText("Subject"), "Hello");
  await user.type(screen.getByLabelText("Email body"), "Here is your link.");
  await user.click(screen.getByRole("button", { name: "Save template" }));
  await screen.findByText("Email template saved.");
  expect(templates.find((t) => t.name === "Follow-up")?.revision).toBe(1);
  expect(
    screen.getByRole<HTMLButtonElement>("button", {
      name: "Delete Pricing & next steps",
    }).disabled,
  ).toBe(true);
  await user.click(screen.getByRole("link", { name: "Back to your flow" }));
  await user.click(
    await screen.findByRole("button", { name: "Step 4: Send an email" }),
  );
  expect(screen.getByRole("option", { name: "Follow-up" })).toBeTruthy();
});

test("the simulator remains local while exercising replies and timers", async () => {
  const user = userEvent.setup();
  await boot();
  await user.click(screen.getByRole("button", { name: "Test flow" }));
  const panel = within(screen.getByRole("region", { name: "Flow test" }));
  await user.click(panel.getByRole("button", { name: "Start test" }));
  await user.type(
    panel.getByLabelText("Simulate an incoming DM"),
    "not an email",
  );
  await user.click(panel.getByRole("button", { name: "Send reply" }));
  expect(panel.getByText("Email not recognized")).toBeTruthy();
  await user.type(
    panel.getByLabelText("Simulate an incoming DM"),
    "me@example.com",
  );
  await user.click(panel.getByRole("button", { name: "Send reply" }));
  await user.click(
    panel.getByRole("button", { name: "Advance clock & continue" }),
  );
  expect(panel.getByText("Flow complete")).toBeTruthy();
  expect(calls.every((c) => c.method === "GET")).toBe(true);
});

test("run-together actions stay expanded and its menu closes outside", async () => {
  const user = userEvent.setup();
  await boot();
  expect(screen.getByText("Sent you a DM! ✨")).toBeTruthy();
  await user.click(screen.getByLabelText("Options for Run together"));
  await user.click(screen.getByRole("heading", { level: 1 }));
  expect(
    screen.getByLabelText("Options for Run together").closest("details")?.open,
  ).toBe(false);
});

test("legacy pinned templates remain available without a library entry", async () => {
  templates = [];
  records[0].draft.workflow.templates = {
    "pricing-intro": { revision: 1, subject: "Pricing", body: "Your link" },
  };
  await boot();
  expect(
    screen.getByRole<HTMLButtonElement>("button", { name: "Publish" }).disabled,
  ).toBe(false);
});

test("reloading a selected automation and returning from templates retains its identity", async () => {
  records = [record("first", "First flow"), record("second", "Second flow")];
  window.history.replaceState(null, "", "/#automations");
  const user = userEvent.setup();
  const view = render(<App />);
  await user.click(
    await screen.findByRole("link", { name: "Edit Second flow" }),
  );
  await screen.findByRole("button", { name: "Second flow" });
  expect(new URLSearchParams(window.location.search).get("automation")).toBe(
    "second",
  );
  view.unmount();
  render(<App />);
  await screen.findByRole("button", { name: "Second flow" });
  await user.click(screen.getByRole("link", { name: "Email templates" }));
  await user.click(
    await screen.findByRole("link", { name: "Back to your flow" }),
  );
  await screen.findByRole("button", { name: "Second flow" });
});

test("an unavailable automation link does not open a different editor", async () => {
  window.history.replaceState(null, "", "/?automation=deleted#builder");
  render(<App />);
  await screen.findByRole("button", { name: "New automation" });
  expect(screen.queryByRole("button", { name: "Save draft" })).toBeNull();
});
