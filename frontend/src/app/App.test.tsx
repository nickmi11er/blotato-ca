import { StrictMode } from "react";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import App from "./App";
import { defaultDraft } from "../feature/automation/model/defaultDraft";
import { storageKey } from "../feature/automation/storage/draftStorage";

test("switching to exact matching resets all to any and saves keywords through blur", async () => {
  const user = userEvent.setup();
  const view = render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
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
  const saved = JSON.parse(localStorage.getItem(storageKey)!);
  expect(saved.draft.keywords).toEqual(["pricing", "guide"]);
  expect(saved.draft.logic).toBe("any");
  view.unmount();
  render(<App />);
  expect(screen.getByRole("button", { name: "Remove guide" })).toBeTruthy();
  expect(screen.getByText("Saved on this device")).toBeTruthy();
});

test("a restored invalid draft cannot be saved until its messages are fixed", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      draft: {
        ...defaultDraft,
        messages: { ...defaultDraft.messages, question: " " },
      },
      savedAt: new Date().toISOString(),
    }),
  );
  const original = localStorage.getItem(storageKey);
  const user = userEvent.setup();
  render(<App />);
  expect(screen.getByText("Not saved yet")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(localStorage.getItem(storageKey)).toBe(original);
  await user.click(screen.getByRole("button", { name: "Edit messages" }));
  const dialog = within(screen.getByRole("dialog"));
  await user.clear(dialog.getByLabelText("DM asking for their email"));
  await user.type(
    dialog.getByLabelText("DM asking for their email"),
    "What is your email?",
  );
  await user.click(dialog.getByRole("button", { name: "Save messages" }));
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(
    JSON.parse(localStorage.getItem(storageKey)!).draft.messages.question,
  ).toBe("What is your email?");
});

test("changing account scope removes its posts and reverting edits clears dirty state", async () => {
  const user = userEvent.setup();
  render(<App />);
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
  expect(screen.getByText("Select at least one post.")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(JSON.parse(localStorage.getItem(storageKey)!).draft.postIds).toEqual([
    "ig_1",
  ]);
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
  await user.selectOptions(screen.getByLabelText("Match type"), "contains");
  expect(screen.getByText("Unsaved changes")).toBeTruthy();
  await user.selectOptions(screen.getByLabelText("Match type"), "whole_word");
  expect(screen.getByText("Saved on this device")).toBeTruthy();
});

test("failed persistence keeps unsaved edits and reports the storage failure", async () => {
  const user = userEvent.setup();
  render(<App />);
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
  const saved = JSON.parse(localStorage.getItem(storageKey)!);
  expect(saved.draft.accountIds).toEqual(["instagram"]);
  expect(saved.draft.postIds).toEqual(["ig_1"]);
});

test("preview evaluates edited comments and the conversation captures an email", async () => {
  const user = userEvent.setup();
  render(<App />);
  await user.click(screen.getByRole("button", { name: "repricing" }));
  expect(screen.getByText("No match this time")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "pricing" }));
  expect(screen.getByText("It’s a match!")).toBeTruthy();
  screen.getByRole("tab", { name: "Test your trigger" }).focus();
  await user.keyboard("{ArrowLeft}");
  const conversationTab = screen.getByRole("tab", { name: "Conversation" });
  expect(conversationTab.getAttribute("aria-selected")).toBe("true");
  await user.click(
    screen.getByRole("button", { name: "Try the conversation" }),
  );
  await user.type(
    screen.getByLabelText("Your test email address"),
    "jordan@example.com",
  );
  await user.click(screen.getByRole("button", { name: "Send test email" }));
  expect(screen.getByText("Email captured")).toBeTruthy();
  expect(
    screen
      .getByRole("link", { name: /example.com\/pricing/ })
      .getAttribute("href"),
  ).toBe(defaultDraft.messages.link);
});

test("returning to the automation list preserves unsaved editor changes", async () => {
  const user = userEvent.setup();
  render(<App />);
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
