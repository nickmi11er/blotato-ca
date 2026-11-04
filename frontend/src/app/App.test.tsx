import { StrictMode } from "react";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import App from "./App";
import { defaultDraft } from "../feature/automation/model/defaultDraft";
import { automationStorageKey } from "../feature/automation/storage/automationStorage";
import { storageKey } from "../feature/automation/storage/draftStorage";

test("switching to exact matching resets all to any and saves keywords through blur", async () => {
  const user = userEvent.setup();
  const view = render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
  await user.selectOptions(
    screen.getByLabelText("When there are multiple"),
    "all",
  );
  await user.selectOptions(screen.getByLabelText("Match type"), "exact");
  const logic = screen.getByLabelText<HTMLSelectElement>(
    "When there are multiple",
  );
  expect(logic.value).toBe("any");
  expect(logic.disabled).toBe(true);
  await user.type(screen.getByRole("textbox", { name: "Keywords" }), "guide");
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  const saved = JSON.parse(localStorage.getItem(automationStorageKey)!)
    .automations[0];
  expect(saved.draft.keywords).toEqual(["pricing", "guide"]);
  expect(saved.draft.logic).toBe("any");
  view.unmount();
  render(<App />);
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
  expect(screen.getByRole("button", { name: "Remove guide" })).toBeTruthy();
  expect(screen.getByText("Saved on this device")).toBeTruthy();
});

test("incomplete drafts can be saved, but cannot be tested until repaired", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      draft: {
        ...defaultDraft,
        steps: [{ id: "bad-dm", kind: "dm", text: " " }],
      },
      savedAt: new Date().toISOString(),
    }),
  );
  const user = userEvent.setup();
  render(<App />);
  expect(screen.getByText("Saved on this device")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .steps[0].text,
  ).toBe(" ");
  await user.click(screen.getByRole("button", { name: "Test flow" }));
  expect(
    screen.getByRole<HTMLButtonElement>("button", { name: "Start test" })
      .disabled,
  ).toBe(true);
  await user.click(
    within(screen.getByRole("region", { name: "Flow test" })).getByRole(
      "button",
      { name: "Exit test" },
    ),
  );
  await user.click(screen.getByRole("button", { name: "Step 1: Send a DM" }));
  await user.clear(screen.getByLabelText("Message"));
  await user.type(screen.getByLabelText("Message"), "What is your email?");
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .steps[0].text,
  ).toBe("What is your email?");
});

test("changing account scope removes its posts and reverting edits clears dirty state", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: /WHEN THIS HAPPENS/ }));
  await user.click(screen.getByRole("button", { name: "Specific posts" }));
  await user.click(
    screen.getByRole("checkbox", {
      name: "Select post: A little idea. A big beginning.",
    }),
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await user.click(
    screen.getByRole("checkbox", { name: "Instagram @northstar.studio" }),
  );

  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .postIds,
  ).toEqual([]);
  await user.click(
    screen.getByRole("checkbox", { name: "Instagram @northstar.studio" }),
  );
  await user.click(
    screen.getByRole("checkbox", {
      name: "Select post: A little idea. A big beginning.",
    }),
  );
  // Account array order is significant in the serialized model; save the new order first.
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  await user.selectOptions(screen.getByLabelText("Match type"), "whole_word");
  expect(screen.getByText("Saved on this device")).toBeTruthy();
});

test("failed persistence keeps unsaved edits and reports the storage failure", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("quota");
  });
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  expect(screen.getByText(/Browser storage is unavailable/)).toBeTruthy();
});

test("unavailable selections remain visible and can be repaired", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      draft: {
        ...defaultDraft,
        accountIds: ["instagram", "deleted-account"],
        postScope: "selected",
        postIds: ["deleted-post"],
      },
    }),
  );
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: /WHEN THIS HAPPENS/ }));
  expect(screen.getByText("Unavailable account: deleted-account")).toBeTruthy();
  expect(screen.getByText("Unavailable post: deleted-post")).toBeTruthy();
  await user.click(
    screen.getByRole("button", {
      name: "Remove unavailable post deleted-post",
    }),
  );
  await user.click(
    screen.getByRole("button", {
      name: "Remove unavailable account deleted-account",
    }),
  );
  await user.click(
    screen.getByRole("checkbox", {
      name: "Select post: A little idea. A big beginning.",
    }),
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  const saved = JSON.parse(localStorage.getItem(automationStorageKey)!)
    .automations[0];
  expect(saved.draft.accountIds).toEqual(["instagram"]);
  expect(saved.draft.postIds).toEqual(["ig_1"]);
});

test("the test runner handles invalid replies, email capture and timers", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: "Test flow" }));
  const dialog = within(screen.getByRole("region", { name: "Flow test" }));
  await user.click(dialog.getByRole("button", { name: "Start test" }));
  await user.type(
    dialog.getByLabelText("Simulate an incoming DM"),
    "not an email",
  );
  await user.click(dialog.getByRole("button", { name: "Send reply" }));
  expect(dialog.getByText("Email not recognized")).toBeTruthy();
  await user.type(
    dialog.getByLabelText("Simulate an incoming DM"),
    "My email is jordan@example.com",
  );
  await user.click(dialog.getByRole("button", { name: "Send reply" }));
  expect(dialog.getByText("Email captured")).toBeTruthy();
  await user.click(
    dialog.getByRole("button", { name: "Advance clock & continue" }),
  );
  expect(dialog.getByText("Flow complete")).toBeTruthy();
  expect(dialog.getByText("Email → jordan@example.com")).toBeTruthy();
});

test("returning to the automation list preserves unsaved editor changes", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  await user.click(screen.getByRole("link", { name: "Automations" }));
  await waitFor(() =>
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(
      "Automations 1",
    ),
  );
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  await user.click(
    screen.getByRole("link", { name: `Edit ${defaultDraft.name}` }),
  );
  await user.click(
    await screen.findByRole("button", { name: /ONLY CONTINUE IF/ }),
  );
  await waitFor(() =>
    expect(screen.getByLabelText<HTMLSelectElement>("Match type").value).toBe(
      "contains",
    ),
  );
  expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(
    defaultDraft.name,
  );
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
});

test("users add a chosen action, configure it, reorder it and remove it", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(
    screen.getByRole("button", { name: "Add step at position 5" }),
  );
  const palette = within(screen.getByRole("dialog"));
  await user.click(palette.getByRole("button", { name: /Send a DM/ }));
  await user.clear(screen.getByLabelText("Message"));
  await user.type(screen.getByLabelText("Message"), "Thanks for connecting!");
  await user.click(screen.getByRole("button", { name: "Move step up" }));
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .steps[3],
  ).toMatchObject({ kind: "dm", text: "Thanks for connecting!" });
  await user.click(screen.getByRole("button", { name: "Remove step" }));
  expect(
    screen.queryByRole("button", { name: "Step 4: Send a DM" }),
  ).toBeNull();
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
});

test("templates can be created and selected by an email step while referenced templates cannot be deleted", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("link", { name: "Email templates" }));
  await user.click(await screen.findByRole("button", { name: "New template" }));
  const dialog = within(screen.getByRole("dialog"));
  await user.type(
    dialog.getByLabelText("Template name"),
    "Interview follow-up",
  );
  await user.type(dialog.getByLabelText("Subject"), "A useful link");
  await user.type(
    dialog.getByLabelText("Email body"),
    "Here is your link: https://example.com/guide",
  );
  await user.click(dialog.getByRole("button", { name: "Save template" }));
  expect(
    screen.getByRole("heading", { name: "Interview follow-up" }),
  ).toBeTruthy();
  expect(
    screen.getByRole<HTMLButtonElement>("button", {
      name: "Delete Pricing & next steps",
    }).disabled,
  ).toBe(true);
  await user.click(screen.getByRole("link", { name: "Back to your flow" }));
  await user.click(
    await screen.findByRole("button", { name: "Step 4: Send an email" }),
  );
  await user.selectOptions(
    screen.getByLabelText("Email template"),
    screen.getByRole("option", { name: "Interview follow-up" }),
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  const id = JSON.parse(localStorage.getItem(automationStorageKey)!)
    .automations[0].draft.steps[3].templateId;
  expect(
    JSON.parse(localStorage.getItem("blotato.email-templates.v1")!).find(
      (t: { id: string }) => t.id === id,
    ).name,
  ).toBe("Interview follow-up");
});

test("an empty parallel group can be saved and deletion can be undone with its configured children intact", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(
    screen.getByRole("button", { name: "Add step at position 5" }),
  );
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: /Run together/,
    }),
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .steps[4].actions,
  ).toEqual([]);
  await user.click(
    screen.getByRole("button", { name: "Add your first action" }),
  );
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: /Send a DM/,
    }),
  );
  expect(screen.getByLabelText<HTMLTextAreaElement>("Message").value).toBe("");
  await user.type(screen.getByLabelText("Message"), "Keep this message");
  await user.click(
    screen.getByRole("button", { name: "Step 5: Run together" }),
  );
  await user.click(screen.getByRole("button", { name: "Remove step" }));
  expect(
    screen.queryByRole("button", { name: "Step 5: Run together" }),
  ).toBeNull();
  await user.click(screen.getByRole("button", { name: "Undo" }));
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .steps[4].actions,
  ).toEqual([expect.objectContaining({ text: "Keep this message" })]);
});

test("outcomes open their own settings and a timeout path can be configured through the canvas", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(
    screen.getByRole("button", { name: "Configure Not an email" }),
  );
  expect(screen.getByLabelText("Retry DM")).toBeTruthy();
  expect(screen.queryByLabelText("Timeout (hours)")).toBeNull();
  await user.click(screen.getByRole("button", { name: /Configure No reply/ }));
  await user.selectOptions(
    screen.getByLabelText("When time runs out"),
    "branch",
  );
  await user.click(screen.getByRole("button", { name: "Add timeout step" }));
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: /Send a DM/,
    }),
  );
  await user.type(screen.getByLabelText("Message"), "You can try again later");
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations[0].draft
      .steps[1].timeoutSteps,
  ).toEqual([
    expect.objectContaining({ kind: "dm", text: "You can try again later" }),
  ]);
  await user.click(screen.getByRole("button", { name: "Test flow" }));
  const panel = within(screen.getByRole("region", { name: "Flow test" }));
  await user.click(panel.getByRole("button", { name: "Start test" }));
  await user.click(panel.getByRole("button", { name: "Simulate timeout" }));
  expect(panel.getByText("You can try again later")).toBeTruthy();
  expect(panel.getByText("Flow complete")).toBeTruthy();
  expect(
    panel.queryByRole("button", { name: "Advance clock & continue" }),
  ).toBeNull();
});

test("step menus dismiss outside, on touch and Escape, while parallel actions start expanded", async () => {
  const user = userEvent.setup();
  render(<App />);
  expect(
    screen.getByRole("button", { name: "Action 1: Reply to comment" }),
  ).toBeTruthy();
  expect(
    screen.getByRole("button", { name: "Action 2: Send a DM" }),
  ).toBeTruthy();
  expect(screen.queryByRole("button", { name: /^Drag / })).toBeNull();
  const options = screen.getByLabelText("Options for Run together");
  const menu = options.closest("details")!;
  await user.click(options);
  expect(menu.open).toBe(true);
  await user.click(screen.getByRole("button", { name: /WHEN THIS HAPPENS/ }));
  expect(menu.open).toBe(false);
  await user.click(options);
  await user.pointer([
    { keys: "[TouchA>]", target: screen.getByRole("heading", { level: 1 }) },
    { keys: "[/TouchA]" },
  ]);
  expect(menu.open).toBe(false);
  await user.click(options);
  await user.keyboard("{Escape}");
  expect(menu.open).toBe(false);
  expect(document.activeElement).toBe(options);
  await user.click(options);
  const waitOptions = screen.getByLabelText("Options for Wait for an email");
  const waitMenu = waitOptions.closest("details")!;
  await user.click(waitOptions);
  expect(waitMenu.open).toBe(true);
  expect(menu.open).toBe(false);
  await user.click(within(waitMenu).getByRole("button", { name: "Duplicate" }));
  expect(
    screen.getByRole("button", { name: "Step 3: Wait for an email" }),
  ).toBeTruthy();
  expect(waitMenu.open).toBe(false);
});

test("new automations are empty, persist independently and keep other unsaved edits", async () => {
  const user = userEvent.setup();
  const view = render(<App />);
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  await user.click(screen.getByRole("link", { name: "Automations" }));
  await user.click(
    await screen.findByRole("button", { name: "New automation" }),
  );
  await screen.findByRole("heading", { name: "Untitled automation" });
  const stored = JSON.parse(localStorage.getItem(automationStorageKey)!);
  expect(stored.automations).toHaveLength(2);
  expect(stored.automations[0].draft.operator).toBe("whole_word");
  expect(stored.automations[1].draft).toMatchObject({
    steps: [],
    keywords: [],
    accountIds: [],
  });
  expect(
    screen.queryByRole("button", { name: "Step 1: Run together" }),
  ).toBeNull();
  await user.click(screen.getByRole("link", { name: "Email templates" }));
  expect(
    (
      await screen.findByRole<HTMLButtonElement>("button", {
        name: "Delete Pricing & next steps",
      })
    ).disabled,
  ).toBe(true);
  await user.click(screen.getByRole("link", { name: "Back to your flow" }));
  await user.click(
    await screen.findByRole("button", { name: "Add step at position 1" }),
  );
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: /Send a DM/,
    }),
  );
  await user.type(
    screen.getByLabelText("Message"),
    "Second automation message",
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await user.click(screen.getByRole("link", { name: "Automations" }));
  await screen.findByRole("heading", { name: "Automations 2" });
  await user.click(
    screen.getByRole("link", { name: `Edit ${defaultDraft.name}` }),
  );
  await user.click(
    await screen.findByRole("button", { name: /ONLY CONTINUE IF/ }),
  );
  expect(screen.getByLabelText<HTMLSelectElement>("Match type").value).toBe(
    "contains",
  );
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  view.unmount();
  render(<App />);
  await user.click(screen.getByRole("link", { name: "Automations" }));
  await user.click(
    await screen.findByRole("link", { name: "Edit Untitled automation" }),
  );
  await user.click(
    await screen.findByRole("button", { name: "Step 1: Send a DM" }),
  );
  expect(screen.getByLabelText<HTMLTextAreaElement>("Message").value).toBe(
    "Second automation message",
  );
});

test("failed creation leaves the list and existing automation intact", async () => {
  window.history.replaceState(null, "", "#automations");
  const user = userEvent.setup();
  render(<App />);
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("quota");
  });
  await user.click(screen.getByRole("button", { name: "New automation" }));
  expect(screen.getByRole("heading", { name: "Automations 1" })).toBeTruthy();
  expect(
    screen.getByRole("link", { name: `Edit ${defaultDraft.name}` }),
  ).toBeTruthy();
  expect(screen.getByText(/Browser storage is unavailable/)).toBeTruthy();
  expect(localStorage.getItem(automationStorageKey)).toBeNull();
});

test("deleting the last automation supports cancellation, stays empty after reload and allows creating again", async () => {
  window.history.replaceState(null, "", "#automations");
  const user = userEvent.setup();
  const view = render(<App />);
  await user.click(
    screen.getByRole("button", { name: `Options for ${defaultDraft.name}` }),
  );
  await user.click(screen.getByRole("button", { name: "Delete automation" }));
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Keep automation",
    }),
  );
  expect(
    screen.getByRole("link", { name: `Edit ${defaultDraft.name}` }),
  ).toBeTruthy();
  expect(localStorage.getItem(automationStorageKey)).toBeNull();
  await user.click(
    screen.getByRole("button", { name: `Options for ${defaultDraft.name}` }),
  );
  await user.click(screen.getByRole("button", { name: "Delete automation" }));
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Delete automation",
    }),
  );
  expect(screen.getByRole("heading", { name: "Automations 0" })).toBeTruthy();
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations,
  ).toEqual([]);
  view.unmount();
  window.history.replaceState(null, "", "#builder");
  render(<App />);
  expect(screen.getByRole("heading", { name: "Automations 0" })).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Email templates" }));
  expect(
    (
      await screen.findByRole<HTMLButtonElement>("button", {
        name: "Delete Pricing & next steps",
      })
    ).disabled,
  ).toBe(false);
  await user.click(screen.getByRole("link", { name: "Back to your flow" }));
  await user.click(
    await screen.findByRole("button", { name: "Create your first automation" }),
  );
  expect(
    await screen.findByRole("heading", { name: "Untitled automation" }),
  ).toBeTruthy();
  expect(
    JSON.parse(localStorage.getItem(automationStorageKey)!).automations,
  ).toHaveLength(1);
});

test("deleting another automation preserves unsaved edits without implicitly saving them", async () => {
  localStorage.setItem(
    automationStorageKey,
    JSON.stringify({
      version: 1,
      activeId: "first",
      automations: [
        {
          id: "first",
          draft: { ...defaultDraft, name: "First" },
          savedAt: null,
        },
        {
          id: "second",
          draft: { ...defaultDraft, name: "Second" },
          savedAt: null,
        },
      ],
    }),
  );
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: /ONLY CONTINUE IF/ }));
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
  const saved = JSON.parse(localStorage.getItem(automationStorageKey)!);
  expect(saved.automations).toHaveLength(1);
  expect(saved.automations[0].draft.operator).toBe("whole_word");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Edit First" }));
  await user.click(
    await screen.findByRole("button", { name: /ONLY CONTINUE IF/ }),
  );
  expect(screen.getByLabelText<HTMLSelectElement>("Match type").value).toBe(
    "contains",
  );
  await user.click(screen.getByRole("link", { name: "Automations" }));
  await user.click(
    await screen.findByRole("button", { name: "Options for First" }),
  );
  await user.click(screen.getByRole("button", { name: "Delete automation" }));
  expect(
    within(screen.getByRole("dialog")).getByText(/has unsaved changes/),
  ).toBeTruthy();
});

test("a failed deletion keeps the automation and confirmation available for retry", async () => {
  window.history.replaceState(null, "", "#automations");
  const user = userEvent.setup();
  render(<App />);
  await user.click(
    screen.getByRole("button", { name: `Options for ${defaultDraft.name}` }),
  );
  await user.click(screen.getByRole("button", { name: "Delete automation" }));
  const dialog = within(screen.getByRole("dialog"));
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("quota");
  });
  await user.click(dialog.getByRole("button", { name: "Delete automation" }));
  expect(dialog.getByRole("alert").textContent).toMatch(
    /storage is unavailable/,
  );
  await user.click(dialog.getByRole("button", { name: "Keep automation" }));
  expect(
    screen.getByRole("link", { name: `Edit ${defaultDraft.name}` }),
  ).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Automations 1" })).toBeTruthy();
});
