import { expect, test } from "vitest";
import {
  createStep,
  decodeWorkflow,
  defaultWorkflow,
  extractEmail,
  type Step,
} from "./workflow";
import { validateWorkflow } from "./workflowValidation";
import { defaultTemplates } from "../../email-templates/model/templates";
import { defaultDraft } from "./defaultDraft";
import { readDraft, storageKey } from "../storage/draftStorage";
import { validateDraft } from "./validation";
import { accounts, posts } from "../fixtures";

test("email capture accepts one address in natural text and rejects ambiguous or malformed input", () => {
  expect(
    extractEmail("You can email me at <jordan+guide@example.co.uk>. Thanks!"),
  ).toBe("jordan+guide@example.co.uk");
  expect(extractEmail("jordan@example.com, jordan@example.com")).toBe(
    "jordan@example.com",
  );
  for (const text of [
    "hello",
    "a@example.com b@example.com",
    "jordan@example",
    "a..b@example.com",
    "a@-example.com",
    "a@example.com..oops",
    "a@example.com/path",
    "a@@example.com",
    "a@b.c",
    "a@example.com_foo",
  ])
    expect(extractEmail(text), text).toBeNull();
});

test("email dependency follows sequence order, including parallel groups", () => {
  const steps = defaultWorkflow();
  expect(validateWorkflow(steps, defaultTemplates)).toEqual([]);
  const email = steps.pop()!;
  steps.unshift(email);
  expect(validateWorkflow(steps, defaultTemplates)).toEqual([
    expect.objectContaining({ stepId: email.id }),
  ]);
  const wait = createStep("wait_reply");
  const group: Step = {
    id: "group",
    kind: "parallel",
    actions: [
      { id: "email", kind: "email", templateId: "pricing-intro" },
      { id: "dm", kind: "dm", text: "Hello" },
    ],
  };
  expect(validateWorkflow([wait, group], defaultTemplates)).toEqual([]);
  expect(validateWorkflow([group, wait], defaultTemplates)).toHaveLength(1);
  expect(validateWorkflow([wait, group], [])).toHaveLength(1);
});

test("plain reply capture does not provide an email", () => {
  const wait = createStep("wait_reply");
  if (wait.kind !== "wait_reply") throw new Error();
  wait.expected = "text";
  expect(
    validateWorkflow(
      [wait, { id: "send", kind: "email", templateId: "pricing-intro" }],
      defaultTemplates,
    ),
  ).toHaveLength(1);
});

test("strict decoding rejects duplicate IDs, unknown kinds and nested waits", () => {
  const steps = defaultWorkflow();
  expect(decodeWorkflow(steps)).toEqual(steps);
  expect(decodeWorkflow([...steps, steps[0]])).toBeNull();
  expect(
    decodeWorkflow([{ id: "unknown", kind: "execute_script" }]),
  ).toBeNull();
  expect(
    decodeWorkflow([
      { id: "group", kind: "parallel", actions: [createStep("wait_reply")] },
    ]),
  ).toBeNull();
});

test("delay and wait limits are finite, bounded integers", () => {
  expect(
    validateWorkflow(
      [{ id: "timer", kind: "delay", minutes: 0 }],
      defaultTemplates,
    ),
  ).toHaveLength(1);
  expect(
    validateWorkflow(
      [{ id: "timer", kind: "delay", minutes: Infinity }],
      defaultTemplates,
    ),
  ).toHaveLength(1);
  const wait = createStep("wait_reply");
  if (wait.kind !== "wait_reply") throw new Error();
  expect(
    validateWorkflow(
      [{ ...wait, maxAttempts: 0, timeoutHours: 721 }],
      defaultTemplates,
    ),
  ).toHaveLength(2);
  expect(
    validateWorkflow(
      [{ id: "date", kind: "schedule", at: "bad", timezone: "bad/zone" }],
      defaultTemplates,
    ),
  ).toHaveLength(2);
});

test("v1 drafts migrate their original messages without replacing them with email delivery", () => {
  const legacy = {
    ...defaultDraft,
    schemaVersion: 1,
    messages: {
      reply: "Sent it!",
      question: "Your email?",
      linkText: "Here is your link",
      link: "https://example.com/original",
    },
  };
  localStorage.setItem(
    "blotato.trigger-draft.v1",
    JSON.stringify({ draft: legacy, savedAt: "2026-01-01T00:00:00Z" }),
  );
  const { draft } = readDraft(defaultDraft);
  expect(draft.schemaVersion).toBe(2);
  expect(draft.steps).toHaveLength(3);
  expect(draft.steps[0]).toMatchObject({
    kind: "parallel",
    actions: [{ text: "Sent it!" }, { text: "Your email?" }],
  });
  expect(draft.steps[2]).toMatchObject({
    kind: "dm",
    text: "Here is your link\nhttps://example.com/original",
  });
  expect(localStorage.getItem(storageKey)).toBeNull();
});

test("an unreadable saved chain is flagged for repair, never silently replaced", () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      draft: { ...defaultDraft, steps: [{ id: "bad", kind: "unknown" }] },
    }),
  );
  expect(readDraft(defaultDraft).draft.steps).toEqual([]);
  expect(
    validateWorkflow(readDraft(defaultDraft).draft.steps, defaultTemplates),
  ).toHaveLength(1);
});

test("waiting sessions require unambiguous reentry and DM-capable accounts", () => {
  const draft = {
    ...defaultDraft,
    reentry: "every" as const,
    accountIds: ["youtube"],
  };
  expect(validateDraft(draft, { accounts, posts })).toHaveLength(2);
  draft.steps = [{ id: "reply", kind: "reply", text: "Hello" }];
  expect(validateDraft(draft, { accounts, posts })).toEqual([]);
});

test("calendar inputs preserve a local minute and reject silently normalized dates", async () => {
  const { localDateToInstant, localDateValue } = await import("./schedule");
  const instant = localDateToInstant("2030-06-15T10:30");
  expect(instant).not.toBeNull();
  expect(localDateValue(instant!)).toBe("2030-06-15T10:30");
  expect(localDateToInstant("2030-02-31T10:30")).toBeNull();
  expect(localDateToInstant("bad")).toBeNull();
});
