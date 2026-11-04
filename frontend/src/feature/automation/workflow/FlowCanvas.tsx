import { useEffect, useState, type RefObject } from "react";
import { Icon } from "../../../shared/ui/Icon";
import type { Draft } from "../model/types";
import type { EmailTemplate } from "../../email-templates/model/templates";
import {
  durationLabel,
  formatSchedule,
  stepTitle,
  walkSteps,
  type Step,
} from "../model/workflow";
import type { WorkflowIssue } from "../model/workflowValidation";
import type { Simulation } from "../model/simulation";
import { stepOptions } from "./StepPalette";
export type Outcome = "success" | "invalid" | "timeout";
export interface AddPosition {
  index: number;
  parent?: string;
  actionsOnly?: boolean;
}
export type StepOperation = "up" | "down" | "duplicate" | "remove";
function summary(step: Step, templates: readonly EmailTemplate[]): string {
  if (step.kind === "reply" || step.kind === "dm")
    return step.text || "Write your message";
  if (step.kind === "email")
    return (
      templates.find((t) => t.id === step.templateId)?.name ??
      "Choose an email template"
    );
  if (step.kind === "delay") return durationLabel(step.minutes);
  if (step.kind === "schedule") return formatSchedule(step);
  if (step.kind === "wait_reply")
    return `Same conversation · up to ${step.timeoutHours}h`;
  if (step.kind !== "parallel") return "";
  return `${step.actions.length} actions · continue when all succeed`;
}
export function FlowCanvas({
  draft,
  selected,
  templates,
  issues,
  select,
  onAdd,
  operate,
  canvasRef,
  run,
  testing = false,
}: {
  draft: Draft;
  selected: string;
  templates: readonly EmailTemplate[];
  issues: WorkflowIssue[];
  select: (id: string, outcome?: Outcome) => void;
  onAdd: (position: AddPosition) => void;
  operate: (id: string, operation: StepOperation) => void;
  canvasRef: RefObject<HTMLElement | null>;
  run: Simulation | null;
  testing?: boolean;
}) {
  const [collapsed, setCollapsed] = useState(() => new Set<string>());
  useEffect(() => {
    const openMenus = () =>
      canvasRef.current?.querySelectorAll<HTMLDetailsElement>(
        ".wf-step-menu[open]",
      );
    const closeOutside = (event: Event) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      openMenus()?.forEach((menu) => {
        if (!menu.contains(target)) menu.open = false;
      });
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      openMenus()?.forEach((menu) => {
        menu.open = false;
        menu.querySelector("summary")?.focus();
      });
    };
    document.addEventListener("pointerdown", closeOutside);
    document.addEventListener("focusin", closeOutside);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOutside);
      document.removeEventListener("focusin", closeOutside);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [canvasRef]);
  const activeId =
    run?.status === "waiting" ? run.sequence[run.cursor]?.id : undefined;
  function toggle(id: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }
  function connector(
    index: number,
    parent?: string,
    actionsOnly = false,
    limit = false,
  ) {
    return (
      <div className="wf-connector">
        {!testing && (
          <button
            disabled={limit || walkSteps(draft.steps).length >= 160}
            aria-label={
              parent
                ? `Add step at position ${index + 1} in timeout path`
                : `Add step at position ${index + 1}`
            }
            title="Add a step"
            onClick={() => onAdd({ index, parent, actionsOnly })}
          >
            <Icon name="plus" />
          </button>
        )}
      </div>
    );
  }
  function controls(step: Step, index: number, length: number, limit: number) {
    if (testing) return null;
    return (
      <div className="wf-node-controls">
        <details className="wf-step-menu">
          <summary
            aria-label={`Options for ${stepTitle(step)}`}
            title="Step options"
          >
            <Icon name="more" />
          </summary>
          <div role="group" aria-label="Step operations">
            {(
              [
                ["up", "up", "Move up"],
                ["down", "down", "Move down"],
                ["duplicate", "copy", "Duplicate"],
                ["remove", "trash", "Delete"],
              ] as const
            ).map(([operation, icon, label]) => (
              <button
                key={operation}
                disabled={
                  operation === "up"
                    ? index === 0
                    : operation === "down"
                      ? index === length - 1
                      : operation === "duplicate"
                        ? length >= limit ||
                          walkSteps(draft.steps).length +
                            walkSteps([step]).length >
                            160
                        : false
                }
                onClick={(e) => {
                  e.currentTarget.closest("details")?.removeAttribute("open");
                  operate(step.id, operation);
                }}
              >
                <Icon name={icon} />
                {label}
              </button>
            ))}
          </div>
        </details>
      </div>
    );
  }
  function card(
    step: Step,
    index: number,
    length: number,
    parent?: string,
    child = false,
  ) {
    const hasIssues = issues.some((issue) =>
      walkSteps([step]).some((s) => s.id === issue.stepId),
    );
    const completed = run?.completedIds.includes(step.id);
    const active = activeId === step.id;
    const visitedOutcome = run?.outcomes[step.id];
    const isWait =
      step.kind === "wait_reply" ||
      step.kind === "delay" ||
      step.kind === "schedule";
    return (
      <div
        key={step.id}
        className={`wf-step-wrap ${child ? "wf-action-wrap" : ""}`}
      >
        <div
          className={`wf-step ${step.kind === "parallel" ? "wf-parallel" : ""} ${active ? "wf-executing" : ""} ${completed ? "wf-executed" : ""}`}
          data-step-id={step.id}
        >
          <div className="wf-node-row">
            <button
              aria-label={`${child ? "Action" : "Step"} ${index + 1}: ${stepTitle(step)}`}
              onClick={() => select(step.id)}
              className={`wf-node ${selected === step.id && !testing ? "selected" : ""} ${isWait ? "wf-wait" : ""}`}
              aria-pressed={selected === step.id}
            >
              <span
                className={`wf-icon ${isWait ? "amber" : step.kind === "email" ? "green" : "violet"}`}
              >
                <Icon
                  name={
                    stepOptions.find((o) => o.kind === step.kind)?.icon ??
                    "bolt"
                  }
                />
              </span>
              <span className="wf-node-copy">
                <strong>{stepTitle(step)}</strong>
                <span>{summary(step, templates)}</span>
              </span>
              {active ? (
                <span className="wf-node-status waiting">Waiting</span>
              ) : completed ? (
                <span className="wf-node-status done">
                  <Icon name="check" /> Done
                </span>
              ) : hasIssues && !testing ? (
                <span className="wf-incomplete">Not configured</span>
              ) : null}
            </button>
            {controls(step, index, length, child ? 8 : 40)}
          </div>
          {step.kind === "parallel" && (
            <div className="wf-parallel-body">
              {step.actions.length > 0 && !testing && (
                <button
                  className="wf-collapse"
                  aria-expanded={!collapsed.has(step.id)}
                  onClick={() => toggle(step.id)}
                >
                  <Icon name="chevron" />
                  {collapsed.has(step.id)
                    ? `Show ${step.actions.length} actions`
                    : "Hide actions"}
                </button>
              )}
              {(!collapsed.has(step.id) || testing) && (
                <div className="wf-parallel-actions">
                  {step.actions.map((action, i) =>
                    card(action, i, step.actions.length, step.id, true),
                  )}
                </div>
              )}
              {!testing && (
                <button
                  className={`wf-add-action ${!step.actions.length ? "wf-empty-group" : ""}`}
                  disabled={
                    step.actions.length >= 8 ||
                    walkSteps(draft.steps).length >= 160
                  }
                  onClick={() => {
                    setCollapsed((prev) => {
                      const next = new Set(prev);
                      next.delete(step.id);
                      return next;
                    });
                    onAdd({
                      index: step.actions.length,
                      parent: step.id,
                      actionsOnly: true,
                    });
                  }}
                >
                  <Icon name="plus" />
                  {step.actions.length
                    ? "Add action to group"
                    : "Add your first action"}
                </button>
              )}
            </div>
          )}
          {step.kind === "wait_reply" && (
            <div className="wf-wait-paths">
              {(
                [
                  [
                    "success",
                    step.expected === "email"
                      ? "Valid email"
                      : "Reply received",
                    "Continue",
                    "check",
                  ],
                  ...(step.expected === "email"
                    ? [
                        [
                          "invalid",
                          "Not an email",
                          step.invalidBehavior === "retry"
                            ? `Ask again · limit ${step.maxAttempts}`
                            : "End run",
                          "branch",
                        ],
                      ]
                    : []),
                  [
                    "timeout",
                    `No reply · ${step.timeoutHours}h`,
                    step.timeoutBehavior === "branch"
                      ? "Timeout path"
                      : step.timeoutBehavior === "dm"
                        ? "Final DM → End"
                        : "End run",
                    "clock",
                  ],
                ] as [Outcome, string, string, "check" | "branch" | "clock"][]
              ).map(([outcome, title, detail, icon]) => (
                <button
                  key={outcome}
                  className={`${outcome === "success" ? "green" : outcome === "invalid" ? "amber" : ""} ${visitedOutcome === outcome ? "wf-outcome-taken" : ""}`}
                  onClick={() => select(step.id, outcome)}
                  aria-label={`Configure ${title}`}
                >
                  <Icon name={icon} />
                  <strong>{title}</strong>
                  <span>{detail}</span>
                  {visitedOutcome === outcome && <small>Taken</small>}
                </button>
              ))}
            </div>
          )}
        </div>
        {step.kind === "wait_reply" && step.timeoutBehavior === "branch" && (
          <section
            className={`wf-timeout-branch ${visitedOutcome === "timeout" ? "wf-branch-taken" : ""}`}
            aria-label={`Timeout path for ${stepTitle(step)}`}
          >
            <div className="wf-branch-title">
              <Icon name="branch" />
              <strong>No reply in {step.timeoutHours}h</strong>
              <span>Separate path</span>
            </div>
            {sequence(step.timeoutSteps ?? [], step.id)}
            <div className="wf-branch-end">End timeout path</div>
          </section>
        )}
        {step.kind === "wait_reply" && step.timeoutBehavior === "branch" && (
          <div className="wf-main-continuation">
            Valid reply → Main path continues
          </div>
        )}
        {!child && connector(index + 1, parent, false, length >= 40)}
      </div>
    );
  }
  function sequence(steps: Step[], parent?: string) {
    return (
      <>
        {connector(0, parent, false, steps.length >= 40)}
        {steps.map((step, index) => card(step, index, steps.length, parent))}
      </>
    );
  }
  return (
    <section
      ref={canvasRef}
      className={`wf-canvas ${testing ? "wf-testing-canvas" : ""}`}
      aria-label="Automation flow"
    >
      <div className="wf-canvas-bar">
        <span>
          <Icon name="branch" />
          {testing ? "Test your flow" : "Your flow"}
          <b>{draft.steps.length} steps</b>
        </span>
        <span>
          {testing
            ? "Follow the highlighted path"
            : "Select a step to configure"}
        </span>
      </div>
      <div className="wf-track">
        <button
          className={`wf-node wf-start ${selected === "trigger" && !testing ? "selected" : ""}`}
          onClick={() => select("trigger")}
        >
          <span className="wf-icon pink">
            <Icon name="comment" />
          </span>
          <span className="wf-node-copy">
            <small>WHEN THIS HAPPENS</small>
            <strong>Someone comments</strong>
            <span>
              {draft.accountIds.length} social accounts ·{" "}
              {draft.postScope === "all"
                ? "All posts"
                : `${draft.postIds.length} selected posts`}
            </span>
          </span>
          {run && (
            <span className="wf-node-status done">
              <Icon name="check" />
            </span>
          )}
        </button>
        <div className="wf-line" />
        <button
          className={`wf-node wf-start ${selected === "rules" && !testing ? "selected" : ""}`}
          onClick={() => select("rules")}
        >
          <span className="wf-icon violet">
            <Icon name="spark" />
          </span>
          <span className="wf-node-copy">
            <small>ONLY CONTINUE IF</small>
            <strong>
              {draft.matchKind === "any" ? "Any comment" : "Comment matches"}
            </strong>
            <span>
              {draft.matchKind === "any"
                ? "Every comment in scope"
                : draft.keywords
                    .map((k) => `“${k}”`)
                    .join(draft.logic === "any" ? " or " : " and ") ||
                  "Add keywords"}
            </span>
          </span>
          {run && <span className="wf-node-status done">Matched</span>}
        </button>
        <div className="wf-filter-out">
          <span>Match ↓</span>
          <span>No match → End</span>
        </div>
        {sequence(draft.steps)}
        <div className="wf-end">
          <span>
            <Icon name="check" />
          </span>
          End of flow
        </div>
      </div>
      <div className="wf-canvas-foot">
        <Icon name="info" />
        {testing
          ? "Simulation only. No messages are sent."
          : "Waiting sessions resume when a reply or timer arrives."}
      </div>
    </section>
  );
}
