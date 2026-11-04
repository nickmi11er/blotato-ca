import type { EmailTemplate } from "../../email-templates/model/templates";
import { decodeWorkflow, type Action, type Step } from "./workflow";

export interface WorkflowIssue {
  stepId: string;
  message: string;
}
export function validateWorkflow(
  steps: Step[],
  templates: readonly EmailTemplate[],
  initialHasEmail = false,
): WorkflowIssue[] {
  const issues: WorkflowIssue[] = [];
  const add = (stepId: string, message: string) =>
    issues.push({ stepId, message });
  if (!decodeWorkflow(steps))
    return [
      {
        stepId: "",
        message:
          "This workflow contains unreadable or duplicate steps. Repair the draft before saving.",
      },
    ];
  if (!steps.length)
    add("", "Add at least one step. A saved workflow may need repair.");
  let hasEmail = initialHasEmail;
  function action(a: Action) {
    if (a.kind === "email") {
      if (!hasEmail)
        add(
          a.id,
          "Send an email needs a successful ‘Wait for an email’ step before it.",
        );
      if (!templates.some((t) => t.id === a.templateId))
        add(a.id, "Choose an available email template.");
    } else if (!a.text.trim() || a.text.length > 4000)
      add(a.id, "Write a message between 1 and 4,000 characters.");
  }
  for (const step of steps) {
    switch (step.kind) {
      case "reply":
      case "dm":
      case "email":
        action(step);
        break;
      case "parallel":
        if (step.actions.length < 2)
          add(step.id, "Run together needs at least two actions.");
        step.actions.forEach(action);
        break;
      case "wait_reply":
        if (
          !Number.isInteger(step.timeoutHours) ||
          step.timeoutHours < 1 ||
          step.timeoutHours > 720
        )
          add(step.id, "Set a reply timeout between 1 and 720 hours.");
        if (step.expected === "email" && step.invalidBehavior === "retry") {
          if (!step.retryMessage.trim())
            add(
              step.id,
              "Write the message to send when an email is not recognized.",
            );
          if (
            !Number.isInteger(step.maxAttempts) ||
            step.maxAttempts < 1 ||
            step.maxAttempts > 10
          )
            add(
              step.id,
              "Allow between 1 and 10 invalid replies before stopping.",
            );
        }
        if (step.timeoutBehavior === "dm" && !step.timeoutMessage.trim())
          add(step.id, "Write the final DM to send on timeout.");
        if (step.timeoutBehavior === "branch") {
          if (!step.timeoutSteps?.length)
            add(
              step.id,
              "Add a step to the timeout path or choose End this run.",
            );
          else
            issues.push(
              ...validateWorkflow(step.timeoutSteps, templates, hasEmail),
            );
        }
        if (step.expected === "email") hasEmail = true;
        break;
      case "delay":
        if (
          !Number.isInteger(step.minutes) ||
          step.minutes < 1 ||
          step.minutes > 43200
        )
          add(step.id, "Set a delay between 1 minute and 30 days.");
        break;
      case "schedule":
        if (
          step.mode === "next_time"
            ? !/^([01]\d|2[0-3]):[0-5]\d$/.test(step.time ?? "")
            : !Number.isFinite(Date.parse(step.at))
        )
          add(step.id, "Choose a date and time to continue.");
        try {
          new Intl.DateTimeFormat("en", { timeZone: step.timezone });
        } catch {
          add(step.id, "Choose a valid timezone.");
        }
        break;
    }
  }
  return issues;
}
