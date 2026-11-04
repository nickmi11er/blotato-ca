import {
  defaultTemplates,
  type EmailTemplate,
} from "../../email-templates/model/templates";
import { validateWorkflow } from "./workflowValidation";
import { walkSteps } from "./workflow";
import { normalize } from "./matcher";
import type { Draft, ScopeCatalog } from "./types";

export function validateDraft(
  draft: Draft,
  catalog: ScopeCatalog,
  templates: readonly EmailTemplate[] = defaultTemplates,
): string[] {
  const errors: string[] = [];
  if (!draft.name.trim()) errors.push("Give your automation a name.");
  if (draft.name.length > 100)
    errors.push("Use a name of at most 100 characters.");
  if (!draft.accountIds.length)
    errors.push("Select at least one social account.");
  if (
    draft.accountIds.some(
      (id) => !catalog.accounts.some((account) => account.id === id),
    )
  )
    errors.push(
      "A selected account is no longer available. Update your account selection.",
    );
  if (draft.postScope === "selected") {
    if (!draft.postIds.length) errors.push("Select at least one post.");
    if (
      draft.postIds.some(
        (id) =>
          !catalog.posts.some(
            (post) =>
              post.id === id && draft.accountIds.includes(post.accountId),
          ),
      )
    )
      errors.push(
        "A selected post is unavailable or belongs to an unselected account. Update your post selection.",
      );
  }
  if (draft.matchKind === "keywords") {
    if (!draft.keywords.length || draft.keywords.some((value) => !value.trim()))
      errors.push("Add at least one non-empty keyword.");
    const distinctKeywords = new Set(
      draft.keywords.map((value) => normalize(value, draft.caseSensitive)),
    );
    if (
      draft.operator === "exact" &&
      draft.logic === "all" &&
      distinctKeywords.size > 1
    )
      errors.push(
        "Exact comments can match any keyword, but cannot match several different keywords at once.",
      );
  }
  if (
    [draft.keywords, draft.exclusions].some(
      (values) =>
        values.length > 30 || values.some((value) => value.length > 120),
    )
  )
    errors.push(
      "Use at most 30 terms per list, each no longer than 120 characters.",
    );
  if (
    draft.reentry === "every" &&
    walkSteps(draft.steps).some((step) => step.kind === "wait_reply")
  )
    errors.push(
      "Flows waiting for replies need one active conversation per person. Change repeat behavior in the trigger settings.",
    );
  const needsDM = walkSteps(draft.steps).some(
    (step) =>
      step.kind === "dm" ||
      step.kind === "wait_reply" ||
      (step.kind === "parallel" &&
        step.actions.some((action) => action.kind === "dm")),
  );
  if (
    needsDM &&
    catalog.accounts.some(
      (account) => draft.accountIds.includes(account.id) && !account.canSendDM,
    )
  )
    errors.push(
      "This flow uses DMs. Select accounts that support direct messages.",
    );
  return [
    ...errors,
    ...validateWorkflow(draft.steps, templates).map((issue) => issue.message),
  ];
}
