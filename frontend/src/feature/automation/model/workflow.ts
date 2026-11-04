export type ActionKind = "reply" | "dm" | "email";
export type Action =
  | { id: string; kind: "reply" | "dm"; text: string }
  | { id: string; kind: "email"; templateId: string };
export interface ReplyWait {
  id: string;
  kind: "wait_reply";
  expected: "email" | "text";
  timeoutHours: number;
  invalidBehavior: "retry" | "stop";
  retryMessage: string;
  maxAttempts: number;
  timeoutBehavior: "stop" | "dm" | "branch";
  timeoutSteps?: Step[];
  timeoutMessage: string;
}
export type Step =
  | Action
  | ReplyWait
  | { id: string; kind: "parallel"; actions: Action[] }
  | { id: string; kind: "delay"; minutes: number }
  | {
      id: string;
      kind: "schedule";
      at: string;
      timezone: string;
      mode?: "date" | "next_time";
      time?: string;
    };
export type StepKind = Step["kind"];
export interface LegacyMessages {
  reply: string;
  question: string;
  linkText: string;
  link: string;
}
export function createStep(kind: StepKind): Step {
  const id = crypto.randomUUID();
  switch (kind) {
    case "reply":
      return { id, kind, text: "" };
    case "dm":
      return {
        id,
        kind,
        text: "",
      };
    case "email":
      return { id, kind, templateId: "" };
    case "parallel":
      return {
        id,
        kind,
        actions: [],
      };
    case "wait_reply":
      return {
        id,
        kind,
        expected: "email",
        timeoutHours: 24,
        invalidBehavior: "retry",
        retryMessage:
          "That doesn’t look like an email address. Could you send it again? For example: you@example.com",
        maxAttempts: 3,
        timeoutBehavior: "stop",
        timeoutMessage:
          "Still interested? Leave a new comment whenever you’re ready.",
      };
    case "delay":
      return { id, kind, minutes: 60 };
    case "schedule":
      return {
        id,
        kind,
        at: "",
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      };
  }
}
export function defaultWorkflow(): Step[] {
  return [
    {
      id: "welcome",
      kind: "parallel",
      actions: [
        { id: "public-reply", kind: "reply", text: "Sent you a DM! ✨" },
        {
          id: "ask-email",
          kind: "dm",
          text: "Hey! Thanks for your interest. What’s the best email address to send our pricing to?",
        },
      ],
    },
    { ...createStep("wait_reply"), id: "capture-email" },
    { id: "follow-up-delay", kind: "delay", minutes: 60 },
    { id: "send-pricing", kind: "email", templateId: "pricing-intro" },
  ];
}
export function migrateMessages(messages: LegacyMessages): Step[] {
  const steps = defaultWorkflow();
  return [
    {
      id: "welcome",
      kind: "parallel",
      actions: [
        { id: "public-reply", kind: "reply", text: messages.reply },
        { id: "ask-email", kind: "dm", text: messages.question },
      ],
    },
    steps[1],
    {
      id: "deliver-link",
      kind: "dm",
      text: `${messages.linkText}\n${messages.link}`,
    },
  ];
}
export function stepTitle(step: Step): string {
  return {
    reply: "Reply to comment",
    dm: "Send a DM",
    email: "Send an email",
    parallel: "Run together",
    wait_reply:
      step.kind === "wait_reply" && step.expected === "text"
        ? "Wait for a reply"
        : "Wait for an email",
    delay: "Wait for a duration",
    schedule:
      step.kind === "schedule" && step.mode === "next_time"
        ? "Wait until a time"
        : "Wait until a date & time",
  }[step.kind];
}
export function durationLabel(minutes: number): string {
  if (minutes >= 1440 && minutes % 1440 === 0)
    return `${minutes / 1440} ${minutes === 1440 ? "day" : "days"}`;
  if (minutes >= 60 && minutes % 60 === 0)
    return `${minutes / 60} ${minutes === 60 ? "hour" : "hours"}`;
  return `${minutes} ${minutes === 1 ? "minute" : "minutes"}`;
}
export function formatSchedule(
  step: Extract<Step, { kind: "schedule" }>,
): string {
  if (step.mode === "next_time")
    return step.time ? `Next ${step.time} · ${step.timezone}` : "Choose a time";
  if (!step.at || !Number.isFinite(Date.parse(step.at)))
    return "Choose a date & time";
  try {
    return (
      new Intl.DateTimeFormat("en", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: step.timezone,
      }).format(new Date(step.at)) + ` · ${step.timezone}`
    );
  } catch {
    return "Choose a valid timezone";
  }
}
// Capture one address from natural text; multiple addresses are intentionally ambiguous.
export function extractEmail(text: string): string | null {
  const candidates = text.match(/[^\s<>()",;:]+@[^\s<>()",;:]+/g) ?? [];
  const unique = [
    ...new Set(candidates.map((value) => value.replace(/[.!?]+$/, ""))),
  ];
  if (unique.length !== 1) return null;
  const email = unique[0];
  const parts = email.split("@");
  if (parts.length !== 2) return null;
  const [local, domain] = parts;
  return email.length <= 254 &&
    local.length <= 64 &&
    /^[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+$/i.test(local) &&
    !local.startsWith(".") &&
    !local.endsWith(".") &&
    !local.includes("..") &&
    /^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,63}$/i.test(domain) &&
    domain.split(".").every((label) => label.length <= 63)
    ? email
    : null;
}
export function walkSteps(steps: readonly Step[]): Step[] {
  return steps.flatMap((step) => [
    step,
    ...(step.kind === "parallel"
      ? step.actions
      : step.kind === "wait_reply"
        ? walkSteps(step.timeoutSteps ?? [])
        : []),
  ]);
}
export function allActions(steps: readonly Step[]): Action[] {
  return walkSteps(steps).filter(
    (step): step is Action =>
      step.kind === "reply" || step.kind === "dm" || step.kind === "email",
  );
}

// Decode persisted configuration with explicit types, limits and IDs. Never drop bad
// nodes silently: the storage layer reports an unreadable workflow instead.
export function decodeWorkflow(value: unknown): Step[] | null {
  if (!Array.isArray(value) || value.length > 40) return null;
  const ids = new Set<string>();
  let count = 0;
  const record = (v: unknown): v is Record<string, unknown> =>
    typeof v === "object" && v !== null && !Array.isArray(v);
  const str = (v: unknown, max = 4000): v is string =>
    typeof v === "string" && v.length <= max;
  function decode(v: unknown, nested = false, depth = 0): Step | null {
    if (depth > 3 || ++count > 160) return null;
    if (!record(v) || !str(v.id, 100) || !v.id || ids.has(v.id)) return null;
    ids.add(v.id);
    const id = v.id;
    if ((v.kind === "reply" || v.kind === "dm") && str(v.text))
      return { id, kind: v.kind, text: v.text };
    if (v.kind === "email" && str(v.templateId, 100))
      return { id, kind: v.kind, templateId: v.templateId };
    if (nested) return null;
    if (
      v.kind === "parallel" &&
      Array.isArray(v.actions) &&
      v.actions.length <= 8
    ) {
      const actions = v.actions.map((a) => decode(a, true, depth));
      return actions.every((a) => a !== null)
        ? { id, kind: "parallel", actions: actions as Action[] }
        : null;
    }
    if (
      v.kind === "delay" &&
      typeof v.minutes === "number" &&
      Number.isFinite(v.minutes)
    )
      return { id, kind: v.kind, minutes: v.minutes };
    if (v.kind === "schedule" && str(v.at, 100) && str(v.timezone, 100)) {
      if (v.mode !== undefined && v.mode !== "date" && v.mode !== "next_time")
        return null;
      if (v.time !== undefined && !str(v.time, 10)) return null;
      return {
        id,
        kind: v.kind,
        at: v.at,
        timezone: v.timezone,
        ...(v.mode ? { mode: v.mode } : {}),
        ...(typeof v.time === "string" ? { time: v.time } : {}),
      };
    }
    if (
      v.kind === "wait_reply" &&
      (v.expected === "email" || v.expected === "text") &&
      typeof v.timeoutHours === "number" &&
      Number.isFinite(v.timeoutHours) &&
      typeof v.maxAttempts === "number" &&
      Number.isFinite(v.maxAttempts) &&
      (v.invalidBehavior === "retry" || v.invalidBehavior === "stop") &&
      (v.timeoutBehavior === "stop" ||
        v.timeoutBehavior === "dm" ||
        v.timeoutBehavior === "branch") &&
      str(v.retryMessage) &&
      str(v.timeoutMessage)
    ) {
      let timeoutSteps: Step[] | undefined;
      if (v.timeoutSteps !== undefined) {
        if (!Array.isArray(v.timeoutSteps) || v.timeoutSteps.length > 40)
          return null;
        const branch = v.timeoutSteps.map((s) => decode(s, false, depth + 1));
        if (branch.some((s) => s === null)) return null;
        timeoutSteps = branch as Step[];
      }
      return {
        id,
        kind: v.kind,
        expected: v.expected,
        timeoutHours: v.timeoutHours,
        invalidBehavior: v.invalidBehavior,
        retryMessage: v.retryMessage,
        maxAttempts: v.maxAttempts,
        timeoutBehavior: v.timeoutBehavior,
        timeoutMessage: v.timeoutMessage,
        ...(timeoutSteps ? { timeoutSteps } : {}),
      };
    }
    return null;
  }
  const result = value.map((v) => decode(v));
  return result.every((s) => s !== null) ? (result as Step[]) : null;
}
