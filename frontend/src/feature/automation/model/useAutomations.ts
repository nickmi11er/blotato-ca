import { useAutomationSelection } from "./useAutomationSelection";
import { useEffect, useRef, useState } from "react";
import type { EmailTemplate } from "../../email-templates/model/templates";
import { automationAPI, type AutomationRecord } from "../api/automations";
import { errorMessage } from "../../../shared/api/client";
import { validateDraft } from "./validation";
import type { Draft, ScopeCatalog, UpdateDraft } from "./types";
interface EditingAutomation extends AutomationRecord {
  savedDraft: Draft;
}
export interface AutomationSummary extends AutomationRecord {
  dirty: boolean;
}
export interface AutomationDraftController {
  draft: Draft;
  update: UpdateDraft;
  toggleAccount: (id: string) => void;
  savedAt: string | null;
  dirty: boolean;
  errors: string[];
  busy: boolean;
  enabled: boolean;
  unpublished: boolean;
  revision: number;
  save: () => Promise<string>;
  publish: () => Promise<string>;
  disable: () => Promise<string>;
}
const isDirty = (entry: EditingAutomation) =>
  JSON.stringify(entry.draft) !== JSON.stringify(entry.savedDraft);
export function useAutomations(
  defaults: Draft,
  catalog: ScopeCatalog,
  templates: readonly EmailTemplate[],
  initial: AutomationRecord[],
) {
  const [entries, setEntries] = useState<EditingAutomation[]>(() =>
    initial.map((e) => ({ ...e, savedDraft: e.draft })),
  );
  const [activeId, setActiveId] = useAutomationSelection(initial[0]?.id ?? "");
  const [busy, setBusy] = useState(false);
  const locked = useRef(false);
  const active = entries.find((e) => e.id === activeId);
  const anyDirty = entries.some(isDirty);
  const availableTemplates = [...templates];
  for (const [id, raw] of Object.entries(active?.snapshots ?? {})) {
    if (
      availableTemplates.some((t) => t.id === id) ||
      !raw ||
      typeof raw !== "object"
    )
      continue;
    const t = raw as Record<string, unknown>;
    if (typeof t.subject === "string" && typeof t.body === "string")
      availableTemplates.push({
        id,
        name: `${id} (saved template)`,
        subject: t.subject,
        body: t.body,
      });
  }
  const errors = active
    ? validateDraft(active.draft, catalog, availableTemplates)
    : [];
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
      previous.map((e) =>
        e.id === activeId
          ? {
              ...e,
              draft: {
                ...e.draft,
                ...patch,
                ...(patch.postScope ? { scopeModes: undefined } : {}),
              },
            }
          : e,
      ),
    );
  }
  function toggleAccount(id: string) {
    if (!active) return;
    const accountIds = active.draft.accountIds.includes(id)
      ? active.draft.accountIds.filter((a) => a !== id)
      : [...active.draft.accountIds, id];
    update({
      accountIds,
      postIds: active.draft.postIds.filter((postId) =>
        catalog.posts.some(
          (p) => p.id === postId && accountIds.includes(p.accountId),
        ),
      ),
    });
  }
  async function operate<T>(operation: () => Promise<T>): Promise<T | string> {
    if (locked.current) return "Another request is still in progress.";
    locked.current = true;
    setBusy(true);
    try {
      return await operation();
    } catch (error) {
      return errorMessage(error);
    } finally {
      locked.current = false;
      setBusy(false);
    }
  }
  async function persist(entry: EditingAutomation) {
    const snapshot = structuredClone(entry.draft);
    await automationAPI.save(
      entry.id,
      snapshot,
      entry.version,
      entry.snapshots,
    );
    const savedAt = new Date().toISOString(),
      version = entry.version + 1;
    // Keep edits typed while the request was in flight; only advance the saved snapshot.
    setEntries((previous) =>
      previous.map((e) =>
        e.id === entry.id
          ? { ...e, savedDraft: snapshot, savedAt, version }
          : e,
      ),
    );
    return version;
  }
  async function save() {
    return operate(async () => {
      if (!active) return "Choose an automation to save.";
      await persist(active);
      return errors.length
        ? "Draft saved. You can finish configuring it later."
        : "Draft saved.";
    });
  }
  async function publish() {
    return operate(async () => {
      if (!active) return "Choose an automation to publish.";
      if (errors.length) return errors[0];
      const version = isDirty(active) ? await persist(active) : active.version;
      await automationAPI.mutate(active.id, "publish", version);
      const latest = await automationAPI.get(active.id);
      setEntries((previous) =>
        previous.map((e) =>
          e.id === active.id
            ? {
                ...e,
                enabled: latest.enabled,
                revision: latest.revision,
                publishedVersion: latest.publishedVersion,
              }
            : e,
        ),
      );
      return "Automation published. New matching comments will start this flow.";
    });
  }
  async function disable() {
    return operate(async () => {
      if (!active) return "Choose an automation.";
      await automationAPI.mutate(active.id, "disable", active.version);
      setEntries((previous) =>
        previous.map((e) =>
          e.id === active.id ? { ...e, enabled: false } : e,
        ),
      );
      return "Automation paused. Existing sessions will continue.";
    });
  }
  async function create(): Promise<string | null> {
    return operate(async () => {
      if (entries.length >= 100) return "You can create up to 100 automations.";
      let name = "Untitled automation";
      for (let n = 2; entries.some((e) => e.draft.name === name); n++)
        name = `Untitled automation ${n}`;
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
      await automationAPI.save(id, draft, 0);
      setEntries((previous) => [
        ...previous,
        {
          id,
          draft,
          savedDraft: draft,
          savedAt: new Date().toISOString(),
          version: 1,
          publishedVersion: 0,
          revision: 0,
          enabled: false,
        },
      ]);
      setActiveId(id);
      return null;
    });
  }
  async function remove(id: string): Promise<string | null> {
    return operate(async () => {
      const entry = entries.find((e) => e.id === id);
      if (!entry) return "This automation no longer exists.";
      await automationAPI.mutate(id, "delete", entry.version);
      setEntries((previous) => previous.filter((e) => e.id !== id));
      if (activeId === id)
        setActiveId(entries.find((e) => e.id !== id)?.id ?? "");
      return null;
    });
  }
  return {
    activeId,
    busy,
    templates: availableTemplates,
    automations: entries.map((e) => ({ ...e, dirty: isDirty(e) })),
    templateSteps: entries.flatMap((e) => [
      ...e.draft.steps,
      ...e.savedDraft.steps,
    ]),
    controller: active
      ? ({
          draft: active.draft,
          update,
          toggleAccount,
          savedAt: active.savedAt,
          dirty: isDirty(active),
          errors,
          busy,
          enabled: active.enabled,
          unpublished: active.version !== active.publishedVersion,
          revision: active.revision,
          save,
          publish,
          disable,
        } satisfies AutomationDraftController)
      : null,
    create,
    remove,
    select: (id: string) => {
      if (entries.some((e) => e.id === id)) setActiveId(id);
    },
  };
}
