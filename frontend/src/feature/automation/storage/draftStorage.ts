import type { Draft, Messages, ScopeCatalog } from "../model/types";
import { validateDraft } from "../model/validation";

export const storageKey = "blotato.trigger-draft.v1";
export interface StoredDraft {
  draft: Draft;
  savedAt: string | null;
}

export function readDraft(defaults: Draft): StoredDraft {
  try {
    const saved: unknown = JSON.parse(
      localStorage.getItem(storageKey) || "null",
    );
    if (
      isRecord(saved) &&
      isRecord(saved.draft) &&
      saved.draft.schemaVersion === 1
    )
      return {
        draft: restoreDraft(saved.draft, defaults),
        savedAt:
          typeof saved.savedAt === "string" &&
          Number.isFinite(Date.parse(saved.savedAt))
            ? saved.savedAt
            : null,
      };
  } catch {
    /* Storage can be unavailable or contain invalid JSON. */
  }
  return { draft: structuredClone(defaults), savedAt: null };
}

export function saveDraft(
  draft: Draft,
  catalog: ScopeCatalog,
): { ok: true; savedAt: string } | { ok: false; error: string } {
  const errors = validateDraft(draft, catalog);
  if (errors.length) return { ok: false, error: errors[0] };
  try {
    const savedAt = new Date().toISOString();
    localStorage.setItem(storageKey, JSON.stringify({ draft, savedAt }));
    return { ok: true, savedAt };
  } catch {
    return {
      ok: false,
      error:
        "Browser storage is unavailable. You can still explore and test your trigger.",
    };
  }
}

// Drafts are browser-local, and older or malformed storage never replaces defaults.
export function restoreDraft(value: unknown, defaults: Draft): Draft {
  const result = structuredClone(defaults);
  if (!isRecord(value) || value.schemaVersion !== 1) return result;
  if (typeof value.name === "string") result.name = value.name.slice(0, 100);
  for (const key of [
    "accountIds",
    "postIds",
    "keywords",
    "exclusions",
  ] as const) {
    if (Array.isArray(value[key]))
      result[key] = [
        ...new Set(
          value[key]
            .filter((entry) => typeof entry === "string" && entry.trim())
            .map((entry) => entry.normalize("NFC").trim().slice(0, 120)),
        ),
      ].slice(0, 30);
  }
  if (value.postScope === "all" || value.postScope === "selected")
    result.postScope = value.postScope;
  if (value.matchKind === "any" || value.matchKind === "keywords")
    result.matchKind = value.matchKind;
  if (
    value.operator === "whole_word" ||
    value.operator === "exact" ||
    value.operator === "contains"
  )
    result.operator = value.operator;
  if (value.logic === "any" || value.logic === "all")
    result.logic = value.logic;
  if (
    value.reentry === "skip_active" ||
    value.reentry === "once" ||
    value.reentry === "every"
  )
    result.reentry = value.reentry;
  if (typeof value.caseSensitive === "boolean")
    result.caseSensitive = value.caseSensitive;
  if (isRecord(value.messages)) {
    for (const key of Object.keys(result.messages) as (keyof Messages)[]) {
      if (typeof value.messages[key] === "string")
        result.messages[key] = value.messages[key].slice(0, 1000);
    }
    try {
      if (!["https:", "http:"].includes(new URL(result.messages.link).protocol))
        result.messages.link = defaults.messages.link;
    } catch {
      result.messages.link = defaults.messages.link;
    }
  }
  return result;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
