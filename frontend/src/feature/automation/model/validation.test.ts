import { expect, test } from "vitest";
import { defaultDraft } from "./defaultDraft";
import { validateDraft } from "./validation";
import { accounts, posts } from "../fixtures";
import { restoreDraft } from "../storage/draftStorage";

const catalog = { accounts, posts };

test("rejects contradictory exact rules while accepting equivalent normalized keywords", () => {
  const draft = {
    ...structuredClone(defaultDraft),
    operator: "exact" as const,
    logic: "all" as const,
    keywords: ["pricing", "guide"],
  };
  expect(validateDraft(draft, catalog)).toContain(
    "Exact comments can match any keyword, but cannot match several different keywords at once.",
  );
  draft.keywords = ["pricing", "PRICING"];
  expect(validateDraft(draft, catalog)).toEqual([]);
  draft.caseSensitive = true;
  expect(validateDraft(draft, catalog)).toHaveLength(1);
});

test("validates restored messages and stale or mismatched scope references", () => {
  const draft = restoreDraft(
    {
      ...defaultDraft,
      postScope: "selected",
      postIds: ["deleted-post"],
      steps: [{ id: "test-dm", kind: "dm", text: " " }],
    },
    defaultDraft,
  );
  expect(validateDraft(draft, catalog)).toHaveLength(2);
  draft.postIds = ["yt_1"];
  expect(validateDraft(draft, catalog)).toHaveLength(2);
  draft.postIds = ["ig_1"];
  draft.steps = [{ id: "test-dm", kind: "dm", text: "Email?" }];
  expect(validateDraft(draft, catalog)).toEqual([]);
  draft.accountIds = ["deleted-account"];
  expect(validateDraft(draft, catalog)).toHaveLength(2);
});

test("accepts independent identities for multiple accounts on the same platform", () => {
  const catalog = {
    accounts: [
      { ...accounts[0], id: "account-one" },
      { ...accounts[0], id: "account-two" },
    ],
    posts: [{ ...posts[0], id: "post-two", accountId: "account-two" }],
  };
  const draft = restoreDraft(
    {
      ...defaultDraft,
      accountIds: ["account-two"],
      postScope: "selected",
      postIds: ["post-two"],
    },
    defaultDraft,
  );
  expect(draft.accountIds).toEqual(["account-two"]);
  expect(validateDraft(draft, catalog)).toEqual([]);
});
