import { expect, test } from "vitest";
import { defaultWorkflow, createStep, type Step } from "./workflow";
import { startSimulation, resumeSimulation } from "./simulation";
import { defaultTemplates } from "../../email-templates/model/templates";

test("parallel actions complete before waiting, then email capture and delay gate the delivery", () => {
  const steps = defaultWorkflow();
  let run = startSimulation(steps, defaultTemplates);
  expect(run).toMatchObject({ cursor: 1, status: "waiting", email: "" });
  expect(run.events.map((e) => e.label)).toEqual([
    "Run together",
    "Public reply",
    "DM · source account",
    "Wait for an email",
  ]);
  run = resumeSimulation(run, defaultTemplates, {
    kind: "reply",
    text: "hey",
  });
  expect(run).toMatchObject({ cursor: 1, status: "waiting", attempts: 1 });
  expect(run.events.at(-1)?.label).toBe("Ask again · DM");
  run = resumeSimulation(run, defaultTemplates, {
    kind: "reply",
    text: "My email is jordan@example.com",
  });
  expect(run).toMatchObject({
    cursor: 2,
    status: "waiting",
    email: "jordan@example.com",
    attempts: 0,
  });
  expect(run.events.some((e) => e.label.startsWith("Email →"))).toBe(false);
  run = resumeSimulation(run, defaultTemplates, { kind: "timer" });
  expect(run.status).toBe("complete");
  expect(
    run.events.find((e) => e.label === "Email → jordan@example.com")?.text,
  ).toContain("https://example.com/pricing");
  expect(resumeSimulation(run, defaultTemplates, { kind: "timer" })).toBe(run);
});

test("invalid reply limits terminate the run without sending a final retry or email", () => {
  const steps = defaultWorkflow();
  let run = startSimulation(steps, defaultTemplates);
  for (let i = 0; i < 3; i++)
    run = resumeSimulation(run, defaultTemplates, {
      kind: "reply",
      text: "not email",
    });
  expect(run.status).toBe("stopped");
  expect(run.events.filter((e) => e.label === "Ask again · DM")).toHaveLength(
    2,
  );
  expect(run.events.some((e) => e.label.startsWith("Email →"))).toBe(false);
});

test("timeout optionally sends a final DM, ends the run and ignores late replies", () => {
  const wait = createStep("wait_reply");
  if (wait.kind !== "wait_reply") throw new Error();
  const steps: Step[] = [
    { ...wait, timeoutBehavior: "dm", timeoutMessage: "See you later" },
    createStep("email"),
  ];
  const run = resumeSimulation(
    startSimulation(steps, defaultTemplates),
    defaultTemplates,
    { kind: "timeout" },
  );
  expect(run.status).toBe("stopped");
  expect(run.events.at(-2)).toMatchObject({
    label: "Final DM · source account",
    text: "See you later",
  });
  expect(
    resumeSimulation(run, defaultTemplates, {
      kind: "reply",
      text: "late@example.com",
    }),
  ).toBe(run);
});

test("a reply cannot bypass a timer and a timer cannot bypass reply capture", () => {
  const steps = defaultWorkflow();
  const run = startSimulation(steps, defaultTemplates);
  expect(resumeSimulation(run, defaultTemplates, { kind: "timer" })).toBe(run);
  const timerSteps = [createStep("delay")];
  const timer = startSimulation(timerSteps, defaultTemplates);
  expect(
    resumeSimulation(timer, defaultTemplates, {
      kind: "reply",
      text: "a@example.com",
    }),
  ).toBe(timer);
});

test("scheduled dates in the past continue immediately, future dates wait", () => {
  const steps: Step[] = [
    {
      id: "time",
      kind: "schedule",
      at: "2030-01-01T12:00:00Z",
      timezone: "UTC",
    },
    { id: "reply", kind: "reply", text: "Done" },
  ];
  expect(
    startSimulation(steps, defaultTemplates, Date.parse("2030-01-01T13:00:00Z"))
      .status,
  ).toBe("complete");
  const run = startSimulation(
    steps,
    defaultTemplates,
    Date.parse("2030-01-01T11:00:00Z"),
  );
  expect(run.status).toBe("waiting");
  expect(
    resumeSimulation(run, defaultTemplates, { kind: "timer" }).status,
  ).toBe("complete");
});

test("any reply accepts non-empty text without fabricating a contact email", () => {
  const wait = createStep("wait_reply");
  if (wait.kind !== "wait_reply") throw new Error();
  const steps: Step[] = [
    { ...wait, expected: "text" },
    { id: "done", kind: "dm", text: "Thank you" },
  ];
  const run = startSimulation(steps, defaultTemplates);
  expect(
    resumeSimulation(run, defaultTemplates, {
      kind: "reply",
      text: " ",
    }),
  ).toBe(run);
  expect(
    resumeSimulation(run, defaultTemplates, {
      kind: "reply",
      text: "Yes please",
    }),
  ).toMatchObject({ status: "complete", email: "" });
});

test("timeout takes its own path, waits again and never rejoins the main continuation", () => {
  const wait = createStep("wait_reply");
  if (wait.kind !== "wait_reply") throw new Error();
  const steps: Step[] = [
    {
      ...wait,
      timeoutBehavior: "branch",
      timeoutSteps: [
        { id: "reminder", kind: "dm", text: "One more chance" },
        { ...wait, id: "capture-again" },
        { id: "branch-email", kind: "email", templateId: "pricing-intro" },
      ],
    },
    { id: "main", kind: "dm", text: "Main path" },
  ];
  const original = startSimulation(steps, defaultTemplates, 0);
  let run = resumeSimulation(
    original,
    defaultTemplates,
    { kind: "timeout" },
    0,
  );
  expect(run.now).toBe(24 * 3600000);
  expect(run.sequence[run.cursor].id).toBe("capture-again");
  expect(run.outcomes[wait.id]).toBe("timeout");
  expect(original.outcomes).toEqual({});
  run = resumeSimulation(
    run,
    defaultTemplates,
    { kind: "reply", text: "a@example.com" },
    0,
  );
  expect(run.status).toBe("complete");
  expect(run.completedIds).toContain("branch-email");
  expect(run.completedIds).not.toContain("main");
  const success = resumeSimulation(
    original,
    defaultTemplates,
    { kind: "reply", text: "a@example.com" },
    0,
  );
  expect(success.completedIds).toContain("main");
  expect(success.completedIds).not.toContain("reminder");
});

test("advancing virtual time changes the next time-of-day calculation", () => {
  const steps: Step[] = [
    { id: "delay", kind: "delay", minutes: 120 },
    {
      id: "morning",
      kind: "schedule",
      mode: "next_time",
      time: "09:00",
      at: "",
      timezone: "UTC",
    },
  ];
  const now = Date.parse("2030-01-01T08:00:00Z");
  let run = startSimulation(steps, defaultTemplates, now);
  run = resumeSimulation(run, defaultTemplates, { kind: "timer" }, now);
  expect(run.now).toBe(Date.parse("2030-01-01T10:00:00Z"));
  expect(run.wakeAt).toBe(Date.parse("2030-01-02T09:00:00Z"));
  run = resumeSimulation(run, defaultTemplates, { kind: "timer" }, now);
  expect(run.status).toBe("complete");
  expect(run.now).toBe(Date.parse("2030-01-02T09:00:00Z"));
});
