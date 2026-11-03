import type { Draft, MatchOperator, MatchResult, TestComment } from "./types";
export function normalize(value: string, caseSensitive = false): string {
  const text = String(value).normalize("NFC").trim();
  return caseSensitive ? text : text.toLowerCase();
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function textMatches(
  text: string,
  value: string,
  operator: MatchOperator,
  caseSensitive = false,
): boolean {
  const source = normalize(text, caseSensitive);
  const term = normalize(value, caseSensitive);
  if (!term) return false;
  if (operator === "exact") return source === term;
  if (operator === "contains") return source.includes(term);
  if (operator !== "whole_word") return false;
  const phrase = term.split(/\s+/u).map(escapeRegExp).join("\\s+");
  return new RegExp(
    `(?:^|[^\\p{L}\\p{N}\\p{M}_])${phrase}(?=$|[^\\p{L}\\p{N}\\p{M}_])`,
    "u",
  ).test(source);
}

export function evaluateTrigger(draft: Draft, event: TestComment): MatchResult {
  const no = (reason: string): MatchResult => ({ matched: false, reason });
  if (!draft.accountIds.includes(event.accountId))
    return no("This account isn’t included in your trigger.");
  if (draft.postScope === "selected" && !draft.postIds.includes(event.postId))
    return no("This post isn’t included in your trigger.");
  if (event.isOwn) return no("Your own comments are always skipped.");
  if (event.activeConversation && draft.reentry === "skip_active")
    return no("This person already has an active conversation.");
  if (event.previouslyStarted && draft.reentry === "once")
    return no("This person has already started this automation.");
  const excluded = draft.exclusions.find((value) =>
    textMatches(event.text, value, "whole_word", draft.caseSensitive),
  );
  if (excluded)
    return no(`Excluded because the comment contains “${excluded}”.`);
  if (draft.matchKind === "any")
    return {
      matched: true,
      reason: "Your trigger accepts any comment from this account and post.",
    };
  if (draft.matchKind !== "keywords")
    return no("This matching mode isn’t available in the preview.");
  if (!draft.keywords.length)
    return no("Add at least one keyword to complete your trigger.");
  const terms = draft.keywords.filter((value) =>
    textMatches(event.text, value, draft.operator, draft.caseSensitive),
  );
  const matched =
    draft.logic === "all"
      ? terms.length === draft.keywords.length
      : terms.length > 0;
  if (!matched)
    return no(
      draft.logic === "all"
        ? "The comment needs to match every keyword in your list."
        : "None of your keywords matched this comment.",
    );
  const modes = {
    whole_word: "a whole word or phrase",
    exact: "the entire comment",
    contains: "part of the comment",
  };
  return {
    matched: true,
    terms,
    reason: `“${terms.join("”, “")}” matched as ${modes[draft.operator]}.`,
  };
}
