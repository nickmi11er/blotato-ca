import { expect, test } from "vitest";
import {
  cloneStep,
  locateStep,
  moveStep,
  replaceStep,
} from "./workflowEditing";
import { createStep, decodeWorkflow, walkSteps, type Step } from "./workflow";
import { validateWorkflow } from "./workflowValidation";
import { defaultTemplates } from "../../email-templates/model/templates";
import { defaultDraft } from "./defaultDraft";
import { readDraft, saveDraft } from "../storage/draftStorage";

function branched(): Step[] {
  const wait = createStep("wait_reply");
  if (wait.kind !== "wait_reply") throw new Error();
  return [
    {
      ...wait,
      timeoutBehavior: "branch",
      timeoutSteps: [
        { id: "reminder", kind: "dm", text: "Still interested?" },
        {
          id: "group",
          kind: "parallel",
          actions: [
            { id: "one", kind: "dm", text: "One" },
            { id: "two", kind: "reply", text: "Two" },
          ],
        },
      ],
    },
    { id: "main", kind: "dm", text: "Main" },
  ];
}

test("nested edits and reorders stay inside their path and do not mutate snapshots", () => {
  const steps = branched();
  const snapshot = structuredClone(steps);
  const edited = replaceStep(steps, { id: "one", kind: "dm", text: "Edited" });
  const moved = moveStep(edited, "one", 1);
  expect(locateStep(moved, "one")).toMatchObject({
    parent: "group",
    index: 1,
    depth: 1,
    actionsOnly: true,
  });
  expect(locateStep(moved, "one")?.step).toMatchObject({ text: "Edited" });
  expect(moved[1]).toBe(steps[1]);
  expect(steps).toEqual(snapshot);
  expect(moveStep(moved, "one", 2)).toBe(moved);
});

test("duplicating a branch assigns fresh IDs to every nested action", () => {
  const steps = branched();
  const clone = cloneStep(steps[0]);
  expect(decodeWorkflow([...steps, clone])).not.toBeNull();
  const oldIds = new Set(walkSteps(steps).map((s) => s.id));
  expect(walkSteps([clone]).every((s) => !oldIds.has(s.id))).toBe(true);
  expect(locateStep([clone], "one")).toBeUndefined();
});

test("timeout paths cannot assume that the timed-out capture supplied an email", () => {
  const steps = branched();
  const wait = steps[0];
  if (wait.kind !== "wait_reply") throw new Error();
  wait.timeoutSteps = [
    { id: "email", kind: "email", templateId: "pricing-intro" },
  ];
  expect(validateWorkflow(steps, defaultTemplates)).toEqual([
    expect.objectContaining({ stepId: "email" }),
  ]);
  expect(
    validateWorkflow([createStep("wait_reply"), ...steps], defaultTemplates),
  ).toEqual([]);
});

test("incomplete nested drafts round-trip; non-serializable numeric values are rejected", () => {
  const steps = branched();
  const wait = steps[0];
  if (wait.kind !== "wait_reply") throw new Error();
  wait.timeoutSteps = [
    createStep("parallel"),
    createStep("dm"),
    createStep("email"),
  ];
  const draft = { ...defaultDraft, steps };
  expect(saveDraft(draft).ok).toBe(true);
  expect(readDraft(defaultDraft).draft).toEqual(draft);
  expect(
    saveDraft({ ...draft, steps: [{ ...wait, maxAttempts: NaN }] }).ok,
  ).toBe(false);
  expect(readDraft(defaultDraft).draft).toEqual(draft);
});
