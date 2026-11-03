import { normalize } from "./matcher";
import type { Draft, Messages, ScopeCatalog } from "./types";

export function validateMessages(messages: Messages): string[] {
  const errors: string[] = [];
  if (Object.values(messages).some((value) => !value.trim()))
    errors.push("Complete each message before saving.");
  if (Object.values(messages).some((value) => value.length > 1000))
    errors.push("Messages and links must be no longer than 1,000 characters.");
  try {
    if (!["http:", "https:"].includes(new URL(messages.link).protocol))
      errors.push("Use an HTTP or HTTPS destination link.");
  } catch {
    errors.push("Enter a valid destination link.");
  }
  return errors;
}

export function validateDraft(draft: Draft, catalog: ScopeCatalog): string[] {
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
  return [...errors, ...validateMessages(draft.messages)];
}
