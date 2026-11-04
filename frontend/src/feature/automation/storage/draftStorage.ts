import type { Draft } from "../model/types";
import { decodeWorkflow, migrateMessages } from "../model/workflow";
export const storageKey = "blotato.trigger-draft.v2";
export interface StoredDraft {
  draft: Draft;
  savedAt: string | null;
}

export function readDraft(defaults: Draft): StoredDraft {
  try {
    const saved: unknown = JSON.parse(
      localStorage.getItem(storageKey) ||
        localStorage.getItem("blotato.trigger-draft.v1") ||
        "null",
    );
    if (
      isRecord(saved) &&
      isRecord(saved.draft) &&
      (saved.draft.schemaVersion === 1 || saved.draft.schemaVersion === 2)
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
): { ok: true; savedAt: string } | { ok: false; error: string } {
  // Incomplete configuration is valid draft data. Only reject unreadable structure.
  if (!decodeWorkflow(draft.steps))
    return {
      ok: false,
      error: "This draft has unreadable steps and cannot be saved.",
    };
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
  if (
    !isRecord(value) ||
    (value.schemaVersion !== 1 && value.schemaVersion !== 2)
  )
    return result;
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
  if (value.schemaVersion === 2) {
    // Invalid nodes require repair; never replace a broken saved chain with actions.
    result.steps = decodeWorkflow(value.steps) ?? [];
  } else if (isRecord(value.messages)) {
    const messages = value.messages;
    result.steps = ["reply", "question", "linkText", "link"].every(
      (key) => typeof messages[key] === "string",
    )
      ? migrateMessages({
          reply: String(messages.reply),
          question: String(messages.question),
          linkText: String(messages.linkText),
          link: String(messages.link),
        })
      : [];
  }
  return result;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
