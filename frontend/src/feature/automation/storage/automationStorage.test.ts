import { expect, test } from "vitest";
import { defaultDraft } from "../model/defaultDraft";
import { storageKey } from "./draftStorage";
import {
  automationStorageKey,
  readAutomations,
  saveAutomations,
} from "./automationStorage";

test("imports the existing draft into the collection without replacing its content or legacy copy", () => {
  const legacy = JSON.stringify({
    draft: { ...defaultDraft, name: "Existing workflow" },
    savedAt: "2026-01-01T00:00:00Z",
  });
  localStorage.setItem(storageKey, legacy);
  const { error, ...collection } = readAutomations(defaultDraft);
  expect(error).toBeNull();
  expect(collection.automations[0].draft.name).toBe("Existing workflow");
  expect(saveAutomations(collection)).toBeNull();
  expect(readAutomations(defaultDraft).automations).toEqual(
    collection.automations,
  );
  expect(localStorage.getItem(storageKey)).toBe(legacy);
});

test("an unreadable collection reports an error instead of silently dropping records", () => {
  const raw = JSON.stringify({
    version: 1,
    automations: [
      { id: "same", draft: defaultDraft },
      { id: "same", draft: defaultDraft },
    ],
  });
  localStorage.setItem(automationStorageKey, raw);
  expect(readAutomations(defaultDraft).error).toMatch(/could not be read/);
  expect(localStorage.getItem(automationStorageKey)).toBe(raw);
});

test("an intentionally empty collection does not reimport a deleted legacy draft", () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ draft: defaultDraft, savedAt: null }),
  );
  expect(saveAutomations({ automations: [], activeId: "" })).toBeNull();
  expect(readAutomations(defaultDraft)).toEqual({
    automations: [],
    activeId: "",
    error: null,
  });
});
