import type { Draft } from "../model/types";
import { decodeWorkflow } from "../model/workflow";
import { readDraft, restoreDraft, type StoredDraft } from "./draftStorage";

export const automationStorageKey = "blotato.automations.v1";
export interface StoredAutomation extends StoredDraft {
  id: string;
}
export interface AutomationCollection {
  automations: StoredAutomation[];
  activeId: string;
}

// Import the original single draft only when no collection has been saved yet.
// Keep the legacy key intact so migration never destroys the previous copy.
export function readAutomations(
  defaults: Draft,
): AutomationCollection & { error: string | null } {
  const fallback = () => ({
    automations: [{ id: "initial", ...readDraft(defaults) }],
    activeId: "initial",
  });
  try {
    const raw = localStorage.getItem(automationStorageKey);
    if (!raw) return { ...fallback(), error: null };
    const stored: unknown = JSON.parse(raw);
    if (
      !isRecord(stored) ||
      stored.version !== 1 ||
      !Array.isArray(stored.automations) ||
      stored.automations.length > 100
    )
      throw new Error("Invalid collection");
    const ids = new Set<string>();
    const automations = stored.automations.map((entry): StoredAutomation => {
      if (
        !isRecord(entry) ||
        typeof entry.id !== "string" ||
        !entry.id ||
        entry.id.length > 100 ||
        ids.has(entry.id) ||
        !isRecord(entry.draft) ||
        entry.draft.schemaVersion !== 2
      )
        throw new Error("Invalid automation");
      ids.add(entry.id);
      return {
        id: entry.id,
        draft: restoreDraft(entry.draft, defaults),
        savedAt:
          typeof entry.savedAt === "string" &&
          Number.isFinite(Date.parse(entry.savedAt))
            ? entry.savedAt
            : null,
      };
    });
    return {
      automations,
      activeId:
        typeof stored.activeId === "string" && ids.has(stored.activeId)
          ? stored.activeId
          : (automations[0]?.id ?? ""),
      error: null,
    };
  } catch {
    return {
      ...fallback(),
      error:
        "Saved automations could not be read. Reload after restoring browser storage; saving is disabled to protect the existing data.",
    };
  }
}
export function saveAutomations(
  collection: AutomationCollection,
): string | null {
  if (
    collection.automations.length > 100 ||
    new Set(collection.automations.map((a) => a.id)).size !==
      collection.automations.length ||
    collection.automations.some((a) => !decodeWorkflow(a.draft.steps))
  )
    return "This collection has unreadable steps and cannot be saved.";
  try {
    localStorage.setItem(
      automationStorageKey,
      JSON.stringify({ version: 1, ...collection }),
    );
    return null;
  } catch {
    return "Browser storage is unavailable. Your changes have not been saved.";
  }
}
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
