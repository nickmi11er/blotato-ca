import type { Draft, ScopeCatalog, Post } from "../model/types";
import { decodeWorkflow, type Step } from "../model/workflow";
import { ApiError, request, versionHeader } from "../../../shared/api/client";

interface Document {
  name: string;
  priority?: number;
  trigger: {
    schema_version: 1;
    scopes: {
      account_id: string;
      posts: { mode: "all" | "selected"; ids?: string[] };
    }[];
    comment: {
      mode: Draft["matchKind"];
      operator: Draft["operator"];
      logic: Draft["logic"];
      keywords: string[];
      exclusions: string[];
      case_sensitive: boolean;
    };
    repeat: { mode: Draft["reentry"] };
  };
  workflow: {
    schema_version?: number;
    steps: Step[];
    templates?: Record<string, unknown>;
  };
}
export interface AutomationRecord {
  id: string;
  draft: Draft;
  savedAt: string;
  version: number;
  publishedVersion: number;
  revision: number;
  enabled: boolean;
  // Keep pinned snapshots on legacy API drafts, even before template CRUD existed.
  snapshots?: Record<string, unknown>;
}
export interface AutomationResponse {
  id: string;
  enabled: boolean;
  revision: number;
  draft_version: number;
  published_draft_version: number;
  updated_at: string;
  draft: Document;
}
export const postKey = (account: string, post: string) =>
  JSON.stringify([account, post]);
export function decodeAutomation(v: AutomationResponse): AutomationRecord {
  const d = v.draft,
    t = d.trigger;
  const scopes = t.scopes ?? [];
  const steps = decodeWorkflow(fillOptionalFields(d.workflow.steps));
  if (
    !steps ||
    (d.workflow.schema_version !== undefined && d.workflow.schema_version !== 1)
  )
    throw new ApiError(
      422,
      `Automation “${d.name}” has an unsupported workflow. Its saved data has not been changed.`,
    );
  const modes = new Set(scopes.map((s) => s.posts.mode));
  return {
    id: v.id,
    enabled: v.enabled,
    revision: v.revision,
    version: v.draft_version,
    publishedVersion: v.published_draft_version,
    savedAt: v.updated_at,
    snapshots: d.workflow.templates,
    draft: {
      schemaVersion: 2,
      name: d.name,
      priority: d.priority ?? 0,
      accountIds: scopes.map((s) => s.account_id),
      postScope: modes.has("selected") ? "selected" : "all",
      ...(modes.size > 1
        ? {
            scopeModes: Object.fromEntries(
              scopes.map((s) => [s.account_id, s.posts.mode]),
            ),
          }
        : {}),
      postIds: scopes.flatMap((s) =>
        (s.posts.ids ?? []).map((id) => postKey(s.account_id, id)),
      ),
      matchKind: t.comment.mode,
      operator: t.comment.operator,
      logic: t.comment.logic,
      keywords: t.comment.keywords ?? [],
      exclusions: t.comment.exclusions ?? [],
      caseSensitive: t.comment.case_sensitive,
      reentry: t.repeat.mode,
      steps,
    },
  };
}
export function encodeDraft(
  d: Draft,
  snapshots?: Record<string, unknown>,
): Document {
  const selected = d.postIds.map((key) => JSON.parse(key) as [string, string]);
  return {
    name: d.name,
    priority: d.priority ?? 0,
    trigger: {
      schema_version: 1,
      scopes: d.accountIds.map((account_id) => ({
        account_id,
        posts:
          (d.scopeModes?.[account_id] ?? d.postScope) === "all"
            ? { mode: "all" }
            : {
                mode: "selected",
                ids: selected
                  .filter(([account]) => account === account_id)
                  .map(([, id]) => id),
              },
      })),
      comment: {
        mode: d.matchKind,
        operator: d.operator,
        logic: d.logic,
        keywords: d.keywords,
        exclusions: d.exclusions,
        case_sensitive: d.caseSensitive,
      },
      repeat: { mode: d.reentry },
    },
    workflow: {
      schema_version: 1,
      steps: d.steps,
      ...(snapshots ? { templates: snapshots } : {}),
    },
  };
}
export const automationAPI = {
  async list(signal?: AbortSignal) {
    return (await request<AutomationResponse[]>("automations", { signal })).map(
      decodeAutomation,
    );
  },
  async save(
    id: string,
    draft: Draft,
    version: number,
    snapshots?: Record<string, unknown>,
  ) {
    await request(`automations/${encodeURIComponent(id)}`, {
      method: "PUT",
      headers: versionHeader(version),
      body: JSON.stringify(encodeDraft(draft, snapshots)),
    });
  },
  async mutate(
    id: string,
    action: "publish" | "disable" | "delete",
    version: number,
  ) {
    await request(
      `automations/${encodeURIComponent(id)}${action === "delete" ? "" : `/${action}`}`,
      {
        method: action === "delete" ? "DELETE" : "POST",
        headers: versionHeader(version),
      },
    );
  },
  async get(id: string) {
    return decodeAutomation(
      await request<AutomationResponse>(
        `automations/${encodeURIComponent(id)}`,
      ),
    );
  },
};
export async function loadCatalog(signal?: AbortSignal): Promise<ScopeCatalog> {
  const data = await request<{
    accounts: ScopeCatalog["accounts"];
    posts: {
      id: string;
      accountId: string;
      title: string;
      publishedAt: string;
    }[];
  }>("catalog", { signal });
  const posts: Post[] = data.posts.map((p) => ({
    id: postKey(p.accountId, p.id),
    accountId: p.accountId,
    title: p.title,
    date: new Date(p.publishedAt).toLocaleDateString("en", {
      month: "short",
      day: "numeric",
    }),
    cover: "violet",
    copy: p.title,
    comments: 0,
  }));
  return { ...data, posts };
}

// Go omits empty optional fields; editor controls need explicit empty values.
function fillOptionalFields(value: unknown): unknown {
  if (!Array.isArray(value)) return value;
  return value.map((item: unknown) => {
    if (!item || typeof item !== "object") return item;
    const s = item as Record<string, unknown>;
    if (s.kind === "parallel")
      return { ...s, actions: fillOptionalFields(s.actions ?? []) };
    if (s.kind === "wait_reply")
      return {
        retryMessage: "",
        timeoutMessage: "",
        maxAttempts: 0,
        ...s,
        ...(s.timeoutSteps
          ? { timeoutSteps: fillOptionalFields(s.timeoutSteps) }
          : {}),
      };
    if (s.kind === "schedule") return { at: "", ...s };
    if (s.kind === "dm" || s.kind === "reply") return { text: "", ...s };
    if (s.kind === "email") return { templateId: "", ...s };
    return s;
  });
}
