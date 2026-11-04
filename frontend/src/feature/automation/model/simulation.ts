import { nextTimeInstant } from "./schedule";
import {
  renderTemplate,
  type EmailTemplate,
} from "../../email-templates/model/templates";
import {
  durationLabel,
  extractEmail,
  formatSchedule,
  stepTitle,
  type Action,
  type Step,
} from "./workflow";

export interface RunEvent {
  stepId?: string;
  label: string;
  text: string;
  tone: "action" | "person" | "wait" | "success" | "stop";
}
export interface Simulation {
  sequence: Step[];
  completedIds: string[];
  outcomes: Record<string, "success" | "invalid" | "timeout">;
  now: number;
  wakeAt?: number;
  cursor: number;
  status: "running" | "waiting" | "complete" | "stopped";
  email: string;
  attempts: number;
  events: RunEvent[];
}
export function startSimulation(
  steps: Step[],
  templates: readonly EmailTemplate[],
  now = Date.now(),
): Simulation {
  return advance(
    {
      sequence: steps,
      completedIds: [],
      outcomes: {},
      now,
      cursor: 0,
      status: "running",
      email: "",
      attempts: 0,
      events: [],
    },
    templates,
    now,
  );
}
function advance(
  state: Simulation,
  templates: readonly EmailTemplate[],
  now: number,
): Simulation {
  const next: Simulation = {
    ...state,
    now,
    completedIds: [...state.completedIds],
    events: [...state.events],
  };
  const sequence = next.sequence;
  let stepId = sequence[next.cursor]?.id;
  const log = (
    label: string,
    text: string,
    tone: RunEvent["tone"] = "action",
  ) => next.events.push({ stepId, label, text, tone });
  function execute(action: Action) {
    if (action.kind === "email") {
      const template = templates.find((t) => t.id === action.templateId);
      if (!template || !next.email) {
        log(
          "Cannot continue",
          "An email address and a valid template are required.",
          "stop",
        );
        next.status = "stopped";
        return false;
      }
      log(
        `Email → ${next.email}`,
        `${renderTemplate(template.subject, next.email)}\n\n${renderTemplate(template.body, next.email)}`,
      );
    } else
      log(
        action.kind === "reply" ? "Public reply" : "DM · source account",
        renderTemplate(action.text, next.email),
      );
    next.completedIds.push(action.id);
    return true;
  }
  while (next.cursor < sequence.length && next.status === "running") {
    const step = sequence[next.cursor];
    stepId = step.id;
    if (
      step.kind === "wait_reply" ||
      step.kind === "delay" ||
      step.kind === "schedule"
    ) {
      const wakeAt =
        step.kind === "delay"
          ? now + step.minutes * 60000
          : step.kind === "schedule"
            ? step.mode === "next_time"
              ? nextTimeInstant(step.time ?? "", step.timezone, now)
              : Date.parse(step.at)
            : undefined;
      next.wakeAt = wakeAt ?? undefined;
      if (step.kind === "schedule" && wakeAt != null && wakeAt <= now) {
        log("Scheduled time has passed", "Continue immediately.");
        next.completedIds.push(step.id);
        next.cursor++;
        continue;
      }
      log(
        stepTitle(step),
        step.kind === "wait_reply"
          ? `Session paused. Listening in the same DM conversation for up to ${step.timeoutHours} hours.`
          : step.kind === "delay"
            ? `Session paused for ${durationLabel(step.minutes)}.`
            : formatSchedule(step),
        "wait",
      );
      next.status = "waiting";
      break;
    }
    let succeeded = true;
    if (step.kind === "parallel") {
      log(
        "Run together",
        "Actions are dispatched independently. Continue after all succeed.",
      );
      for (const action of step.actions) {
        if (!execute(action)) {
          succeeded = false;
          break;
        }
      }
    } else succeeded = execute(step);
    if (succeeded && !next.completedIds.includes(step.id))
      next.completedIds.push(step.id);
    next.cursor++;
  }
  if (next.status === "running") {
    next.status = "complete";
    log("Flow complete", "Every step finished in this simulation.", "success");
  }
  return next;
}
export function resumeSimulation(
  state: Simulation,
  templates: readonly EmailTemplate[],
  event: { kind: "reply"; text: string } | { kind: "timeout" | "timer" },
  now = Date.now(),
): Simulation {
  if (state.status !== "waiting") return state;
  const step = state.sequence[state.cursor];
  if (!step) return state;
  const next = {
    ...state,
    completedIds: [...state.completedIds],
    outcomes: { ...state.outcomes },
    events: [...state.events],
  };
  const log = (label: string, text: string, tone: RunEvent["tone"]) =>
    next.events.push({ stepId: step.id, label, text, tone });
  if (step.kind === "wait_reply") {
    if (event.kind === "timer") return state;
    if (event.kind === "timeout") {
      next.outcomes[step.id] = "timeout";
      if (step.timeoutBehavior === "branch") {
        log(
          "Timeout path",
          "No reply in time. Continuing along the timeout path.",
          "wait",
        );
        return advance(
          {
            ...next,
            sequence: step.timeoutSteps ?? [],
            cursor: 0,
            attempts: 0,
            status: "running",
          },
          templates,
          next.now + step.timeoutHours * 3600000,
        );
      }
      if (step.timeoutBehavior === "dm")
        log("Final DM · source account", step.timeoutMessage, "action");
      log(
        "Session ended",
        "The reply window expired. Later messages will not resume this run.",
        "stop",
      );
      return { ...next, status: "stopped" };
    }
    if (event.kind !== "reply" || !event.text.trim()) return state;
    log("Incoming DM", event.text, "person");
    const email = extractEmail(event.text);
    if (step.expected === "email" && !email) {
      next.outcomes[step.id] = "invalid";
      next.attempts++;
      if (
        step.invalidBehavior === "stop" ||
        next.attempts >= step.maxAttempts
      ) {
        log(
          "Session ended",
          step.invalidBehavior === "stop"
            ? "The configured policy ends this run when an email is not recognized."
            : "The invalid-reply limit was reached.",
          "stop",
        );
        return { ...next, status: "stopped" };
      }
      log(
        "Email not recognized",
        `Invalid reply ${next.attempts} of ${step.maxAttempts}. The original timeout still applies.`,
        "wait",
      );
      log("Ask again · DM", step.retryMessage, "action");
      return next;
    }
    next.outcomes[step.id] = "success";
    if (step.expected === "email" && email) {
      next.email = email;
      log("Email captured", `contact.email = ${email}`, "success");
    } else log("Reply received", "Continue with the next step.", "success");
  } else if (
    (step.kind === "delay" || step.kind === "schedule") &&
    event.kind === "timer"
  )
    log(
      "Clock advanced",
      "The waiting period ended in this simulation.",
      "success",
    );
  else return state;
  next.completedIds.push(step.id);
  return advance(
    { ...next, cursor: next.cursor + 1, attempts: 0, status: "running" },
    templates,
    Math.max(next.now, event.kind === "timer" ? (next.wakeAt ?? now) : now),
  );
}
