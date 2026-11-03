import { useEffect, useState } from "react";
import { readDraft, saveDraft } from "../storage/draftStorage";
import { validateDraft } from "./validation";
import type { Draft, ScopeCatalog } from "./types";

export function useAutomationDraft(defaults: Draft, catalog: ScopeCatalog) {
  const [initial] = useState(() => readDraft(defaults));
  const [draft, setDraft] = useState(initial.draft);
  const [savedAt, setSavedAt] = useState(() =>
    validateDraft(initial.draft, catalog).length ? null : initial.savedAt,
  );
  const [savedSnapshot, setSavedSnapshot] = useState(() =>
    JSON.stringify(initial.draft),
  );
  const dirty = JSON.stringify(draft) !== savedSnapshot;
  const errors = validateDraft(draft, catalog);

  useEffect(() => {
    if (!dirty) return;
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [dirty]);

  function update(patch: Partial<Draft>) {
    setDraft((previous) => ({ ...previous, ...patch }));
  }

  function toggleAccount(id: string) {
    setDraft((previous) => {
      const accountIds = previous.accountIds.includes(id)
        ? previous.accountIds.filter((entry) => entry !== id)
        : [...previous.accountIds, id];
      return {
        ...previous,
        accountIds,
        postIds: previous.postIds.filter((postId) =>
          catalog.posts.some(
            (post) => post.id === postId && accountIds.includes(post.accountId),
          ),
        ),
      };
    });
  }

  function save(): string {
    const result = saveDraft(draft, catalog);
    if (!result.ok) return result.error;
    setSavedAt(result.savedAt);
    setSavedSnapshot(JSON.stringify(draft));
    return "Draft saved on this device. Your next conversation is taking shape.";
  }

  return { draft, update, toggleAccount, savedAt, dirty, errors, save };
}

export type AutomationDraftController = ReturnType<typeof useAutomationDraft>;
