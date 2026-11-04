import { useEffect, useState } from "react";
import type { EmailTemplate } from "../../email-templates/model/templates";
import {
  readAutomations,
  saveAutomations,
  type StoredAutomation,
} from "../storage/automationStorage";
import { validateDraft } from "./validation";
import type { Draft, ScopeCatalog, UpdateDraft } from "./types";

interface EditingAutomation extends StoredAutomation {
  savedDraft: Draft;
}
export interface AutomationSummary extends StoredAutomation {
  dirty: boolean;
}
export interface AutomationDraftController {
  draft: Draft;
  update: UpdateDraft;
  toggleAccount: (id: string) => void;
  savedAt: string | null;
  dirty: boolean;
  errors: string[];
  save: () => string;
}
const isDirty = (entry: EditingAutomation) =>
  JSON.stringify(entry.draft) !== JSON.stringify(entry.savedDraft);

export function useAutomations(
  defaults: Draft,
  catalog: ScopeCatalog,
  templates: readonly EmailTemplate[],
) {
  const [initial] = useState(() => readAutomations(defaults));
  const [entries, setEntries] = useState<EditingAutomation[]>(() =>
    initial.automations.map((entry) => ({ ...entry, savedDraft: entry.draft })),
  );
  const [activeId, setActiveId] = useState(initial.activeId);
  const active = entries.find((entry) => entry.id === activeId);
  const dirty = active ? isDirty(active) : false;
  const anyDirty = entries.some(isDirty);
  const errors = active ? validateDraft(active.draft, catalog, templates) : [];
  useEffect(() => {
    if (!anyDirty) return;
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [anyDirty]);

  function update(patch: Partial<Draft>) {
    setEntries((previous) =>
      previous.map((entry) =>
        entry.id === activeId
          ? { ...entry, draft: { ...entry.draft, ...patch } }
          : entry,
      ),
    );
  }
  function toggleAccount(id: string) {
    if (!active) return;
    const draft = active.draft;
    const accountIds = draft.accountIds.includes(id)
      ? draft.accountIds.filter((entry) => entry !== id)
      : [...draft.accountIds, id];
    update({
      accountIds,
      postIds: draft.postIds.filter((postId) =>
        catalog.posts.some(
          (post) => post.id === postId && accountIds.includes(post.accountId),
        ),
      ),
    });
  }
  function persist(next: EditingAutomation[], nextActiveId: string) {
    if (initial.error) return initial.error;
    // Saving one automation must not implicitly save edits in other automations.
    return saveAutomations({
      activeId: nextActiveId,
      automations: next.map(({ id, savedDraft, savedAt }) => ({
        id,
        draft: savedDraft,
        savedAt,
      })),
    });
  }
  function save() {
    if (!active) return "Choose an automation to save.";
    const savedAt = new Date().toISOString();
    const next = entries.map((entry) =>
      entry.id === activeId
        ? { ...entry, savedAt, savedDraft: entry.draft }
        : entry,
    );
    const error = persist(next, activeId);
    if (error) return error;
    setEntries(next);
    return errors.length
      ? "Draft saved. You can finish configuring it later."
      : "Draft saved on this device.";
  }
  function create(): string | null {
    if (entries.length >= 100)
      return "You can keep up to 100 automations on this device.";
    let name = "Untitled automation";
    for (
      let number = 2;
      entries.some((entry) => entry.draft.name === name);
      number++
    )
      name = `Untitled automation ${number}`;
    const draft: Draft = {
      ...structuredClone(defaults),
      name,
      accountIds: [],
      postIds: [],
      keywords: [],
      exclusions: [],
      steps: [],
    };
    const id = crypto.randomUUID();
    const next = [
      ...entries,
      { id, draft, savedDraft: draft, savedAt: new Date().toISOString() },
    ];
    const error = persist(next, id);
    if (error) return error;
    setEntries(next);
    setActiveId(id);
    return null;
  }
  function remove(id: string): string | null {
    const next = entries.filter((entry) => entry.id !== id);
    if (next.length === entries.length)
      return "This automation no longer exists.";
    const nextActiveId = id === activeId ? (next[0]?.id ?? "") : activeId;
    const error = persist(next, nextActiveId);
    if (error) return error;
    setEntries(next);
    setActiveId(nextActiveId);
    return null;
  }
  return {
    activeId,
    automations: entries.map((entry) => ({
      id: entry.id,
      draft: entry.draft,
      savedAt: entry.savedAt,
      dirty: isDirty(entry),
    })),
    templateSteps: entries.flatMap((entry) => [
      ...entry.draft.steps,
      ...entry.savedDraft.steps,
    ]),
    storageError: initial.error,
    controller: active
      ? ({
          draft: active.draft,
          update,
          toggleAccount,
          savedAt: active.savedAt,
          dirty,
          errors,
          save,
        } satisfies AutomationDraftController)
      : null,
    create,
    remove,
    select: (id: string) => {
      if (entries.some((entry) => entry.id === id)) setActiveId(id);
    },
  };
}
