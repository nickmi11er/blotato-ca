import { defaultDraft } from "./defaultDraft";
import { restoreDraft } from "../storage/draftStorage";
import { validateDraft } from "./validation";
import { accounts, posts } from "../fixtures";
import assert from "node:assert/strict";
import { test } from "vitest";
import { evaluateTrigger, textMatches } from "./matcher.ts";
import type { TestComment } from "./types.ts";

const catalog = { accounts, posts };

const comment: TestComment = {
  accountId: "instagram",
  postId: "ig_1",
  text: "Send pricing please!",
  isOwn: false,
  activeConversation: false,
  previouslyStarted: false,
};

test("whole words match punctuation and case while skipping longer words", () => {
  for (const text of [
    "pricing",
    " PRICING ",
    "Send pricing please!",
    "pricing? ✨",
  ])
    assert.equal(textMatches(text, "pricing", "whole_word"), true, text);
  for (const text of ["repricing", "pricingguide", "pre_pricing", "How much?"])
    assert.equal(textMatches(text, "pricing", "whole_word"), false, text);
  assert.equal(textMatches("repricing", "pricing", "contains"), true);
});

test("exact matching preserves punctuation and supports case sensitivity", () => {
  assert.equal(textMatches("  PRICING ", "pricing", "exact"), true);
  assert.equal(textMatches("pricing?", "pricing", "exact"), false);
  assert.equal(textMatches("PRICING", "pricing", "exact", true), false);
});

test("whole phrases support whitespace and Unicode word boundaries", () => {
  assert.equal(
    textMatches("Send the price   list!", "price list", "whole_word"),
    true,
  );
  assert.equal(textMatches("price, list", "price list", "whole_word"), false);
  assert.equal(textMatches("Какая цена?", "цена", "whole_word"), true);
  assert.equal(textMatches("Переоценах", "цена", "whole_word"), false);
  assert.equal(textMatches("cafe\u0301?", "café", "whole_word"), true);
  assert.equal(textMatches("pricing+ please", "pricing+", "whole_word"), true);
});

test("account and post scopes are enforced before matching", () => {
  const draft = structuredClone(defaultDraft);
  assert.equal(evaluateTrigger(draft, comment).matched, true);
  assert.equal(
    evaluateTrigger(draft, { ...comment, accountId: "youtube" }).matched,
    false,
  );
  draft.postScope = "selected";
  draft.postIds = ["ig_2"];
  assert.equal(evaluateTrigger(draft, comment).matched, false);
  assert.equal(
    evaluateTrigger(draft, { ...comment, postId: "ig_2" }).matched,
    true,
  );
});

test("all and any keywords produce different decisions and exclusions win", () => {
  const draft = structuredClone(defaultDraft);
  draft.keywords.push("guide");
  assert.equal(evaluateTrigger(draft, comment).matched, true);
  draft.logic = "all";
  assert.equal(evaluateTrigger(draft, comment).matched, false);
  assert.equal(
    evaluateTrigger(draft, { ...comment, text: "pricing guide" }).matched,
    true,
  );
  draft.logic = "any";
  draft.exclusions = ["spam"];
  assert.equal(
    evaluateTrigger(draft, { ...comment, text: "pricing spam" }).matched,
    false,
  );
  draft.matchKind = "any";
  assert.equal(
    evaluateTrigger(draft, { ...comment, text: "spam" }).matched,
    false,
  );
  assert.equal(
    evaluateTrigger(draft, { ...comment, text: "hello" }).matched,
    true,
  );
});

test("own comments are skipped and reentry policies stay distinct", () => {
  const draft = structuredClone(defaultDraft);
  assert.equal(
    evaluateTrigger(draft, { ...comment, isOwn: true }).matched,
    false,
  );
  assert.equal(
    evaluateTrigger(draft, { ...comment, activeConversation: true }).matched,
    false,
  );
  assert.equal(
    evaluateTrigger(draft, { ...comment, previouslyStarted: true }).matched,
    true,
  );
  draft.reentry = "once";
  assert.equal(
    evaluateTrigger(draft, { ...comment, previouslyStarted: true }).matched,
    false,
  );
  draft.reentry = "every";
  assert.equal(
    evaluateTrigger(draft, {
      ...comment,
      activeConversation: true,
      previouslyStarted: true,
    }).matched,
    true,
  );
});

test("incomplete drafts cannot be saved", () => {
  const draft = structuredClone(defaultDraft);
  assert.deepEqual(validateDraft(draft, catalog), []);
  draft.accountIds = [];
  draft.keywords = [];
  draft.name = " ";
  assert.equal(validateDraft(draft, catalog).length, 3);
});

test("restored drafts keep supported settings and reject executable links", () => {
  const draft = restoreDraft(
    {
      ...defaultDraft,
      operator: "execute",
      accountIds: ["instagram", "unknown"],
      messages: { link: "javascript:alert(1)" },
    },
    defaultDraft,
  );
  assert.equal(draft.operator, "whole_word");
  assert.deepEqual(draft.accountIds, ["instagram", "unknown"]);
  assert.ok(validateDraft(draft, catalog).length > 0);
  assert.equal(draft.messages.link, defaultDraft.messages.link);
  assert.deepEqual(restoreDraft(null, defaultDraft), defaultDraft);
  assert.deepEqual(
    restoreDraft({ schemaVersion: 99 }, defaultDraft),
    defaultDraft,
  );
});
