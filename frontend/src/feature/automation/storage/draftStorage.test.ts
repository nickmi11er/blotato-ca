import { expect, test, vi } from "vitest";
import { defaultDraft } from "../model/defaultDraft";
import { readDraft, saveDraft, storageKey } from "./draftStorage";

test("round-trips incomplete drafts while refusing structurally unreadable steps", () => {
  const draft = { ...structuredClone(defaultDraft), name: "My automation" };
  const result = saveDraft(draft);
  expect(result.ok).toBe(true);
  expect(readDraft(defaultDraft).draft).toEqual(draft);
  draft.steps = [{ id: "test", kind: "dm", text: " " }];
  expect(saveDraft(draft).ok).toBe(true);
  expect(readDraft(defaultDraft).draft.steps).toEqual(draft.steps);
  const saved = localStorage.getItem(storageKey);
  draft.steps = [draft.steps[0], draft.steps[0]];
  expect(saveDraft(draft).ok).toBe(false);
  expect(localStorage.getItem(storageKey)).toBe(saved);
});

test("handles malformed, unsupported, and unavailable storage", () => {
  localStorage.setItem(storageKey, "not json");
  expect(readDraft(defaultDraft)).toEqual({
    draft: defaultDraft,
    savedAt: null,
  });
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      draft: { schemaVersion: 99 },
      savedAt: new Date().toISOString(),
    }),
  );
  expect(readDraft(defaultDraft).savedAt).toBeNull();
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
    throw new Error("blocked");
  });
  expect(readDraft(defaultDraft).draft).toEqual(defaultDraft);
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("quota");
  });
  expect(saveDraft(defaultDraft)).toMatchObject({ ok: false });
});
