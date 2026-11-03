import { expect, test, vi } from "vitest";
import { defaultDraft } from "../model/defaultDraft";
import { accounts, posts } from "../fixtures";
import { readDraft, saveDraft, storageKey } from "./draftStorage";

const catalog = { accounts, posts };

test("round-trips a valid draft and refuses to overwrite it with invalid data", () => {
  const draft = { ...structuredClone(defaultDraft), name: "My automation" };
  const result = saveDraft(draft, catalog);
  expect(result.ok).toBe(true);
  expect(readDraft(defaultDraft).draft).toEqual(draft);
  const saved = localStorage.getItem(storageKey);
  draft.messages.reply = " ";
  expect(saveDraft(draft, catalog).ok).toBe(false);
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
  expect(saveDraft(defaultDraft, catalog)).toMatchObject({ ok: false });
});
