import { expect, test } from "vitest";
import { encodeDraft, decodeAutomation, postKey } from "./automations";
import { defaultDraft } from "../model/defaultDraft";

test("post identities retain their account scope, including equal provider IDs", () => {
  const draft = {
    ...defaultDraft,
    accountIds: ["one", "two"],
    postScope: "selected" as const,
    postIds: [postKey("one", "same"), postKey("two", "same")],
  };
  const document = encodeDraft(draft);
  expect(document.trigger.scopes).toEqual([
    { account_id: "one", posts: { mode: "selected", ids: ["same"] } },
    { account_id: "two", posts: { mode: "selected", ids: ["same"] } },
  ]);
  const result = decodeAutomation({
    id: "x",
    enabled: true,
    revision: 1,
    draft_version: 2,
    published_draft_version: 1,
    updated_at: "now",
    draft: document,
  });
  expect(result.draft.postIds).toEqual(draft.postIds);
});
test("mixed scopes, priority and explicit prompts survive a round trip", () => {
  const draft = {
    ...defaultDraft,
    priority: 42,
    accountIds: ["one", "two"],
    postScope: "selected" as const,
    scopeModes: { one: "all" as const, two: "selected" as const },
    postIds: [postKey("two", "same")],
  };
  const document = encodeDraft(draft);
  const wait = document.workflow.steps.find((s) => s.kind === "wait_reply")!;
  if (wait.kind === "wait_reply") wait.prompt_node_id = "ask-email";
  const result = decodeAutomation({
    id: "x",
    enabled: true,
    revision: 1,
    draft_version: 2,
    published_draft_version: 1,
    updated_at: "now",
    draft: document,
  });
  expect(encodeDraft(result.draft)).toEqual(document);
});

test("optional Go fields are restored for controlled editor inputs", () => {
  const document = encodeDraft(defaultDraft);
  document.workflow = JSON.parse(
    '{"steps":[{"id":"capture","kind":"wait_reply","expected":"text","timeoutHours":1,"invalidBehavior":"stop","timeoutBehavior":"stop"}]}',
  );
  const result = decodeAutomation({
    id: "x",
    enabled: false,
    revision: 0,
    draft_version: 1,
    published_draft_version: 0,
    updated_at: "now",
    draft: document,
  });
  expect(result.draft.steps[0]).toMatchObject({
    retryMessage: "",
    timeoutMessage: "",
    maxAttempts: 0,
  });
});
