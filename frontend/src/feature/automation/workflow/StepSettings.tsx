import { useState } from "react";
import { Icon } from "../../../shared/ui/Icon";
import {
  renderTemplate,
  type EmailTemplate,
} from "../../email-templates/model/templates";
import { formatSchedule, type Step } from "../model/workflow";
import { localDateValue, localDateToInstant } from "../model/schedule";
import type { Outcome } from "./FlowCanvas";
export function StepSettings({
  step,
  change,
  templates,
  outcome,
  onOutcome,
  depth = 0,
  onAddTimeout,
}: {
  step: Step;
  change: (step: Step) => void;
  templates: readonly EmailTemplate[];
  outcome: Outcome;
  onOutcome: (outcome: Outcome) => void;
  depth?: number;
  onAddTimeout: () => void;
}) {
  if (step.kind === "parallel")
    return (
      <div className="wf-fields">
        <div className="wf-tip">
          <Icon name="parallel" />
          <p>
            Add at least two actions to run together. The flow continues after{" "}
            <strong>all actions succeed</strong>.
          </p>
        </div>
        <p className="wf-help">
          Choose an action on the canvas to write its message or select a
          template.
        </p>
      </div>
    );
  if (step.kind === "reply" || step.kind === "dm") {
    const example =
      step.kind === "reply"
        ? "Sent you a DM! ✨"
        : "Hey {{first_name}}! What’s the best email address to send our pricing to?";
    return (
      <div className="wf-fields">
        <div className="wf-tip">
          <Icon name={step.kind === "dm" ? "send" : "comment"} />
          <p>
            {step.kind === "dm"
              ? "Sent to the commenter through the social account where the comment arrived."
              : "A public reply to the comment that started this run."}
          </p>
        </div>
        <label>
          Message
          <textarea
            rows={5}
            maxLength={4000}
            value={step.text}
            placeholder={example}
            onChange={(e) => change({ ...step, text: e.target.value })}
          />
        </label>
        {!step.text && (
          <button
            className="wf-example"
            onClick={() => change({ ...step, text: example })}
          >
            <Icon name="spark" /> Use example message
          </button>
        )}
        <small className="wf-help">Personalize with {"{{first_name}}"}.</small>
      </div>
    );
  }
  if (step.kind === "email") {
    const template = templates.find((t) => t.id === step.templateId);
    return (
      <div className="wf-fields">
        <div className="wf-variable">
          <Icon name="mail" /> contact.email
          <span>Collected earlier in this flow</span>
        </div>
        <label>
          Email template
          <select
            value={step.templateId}
            onChange={(e) => change({ ...step, templateId: e.target.value })}
          >
            <option value="">Choose a template</option>
            {!template && step.templateId && (
              <option value={step.templateId}>Unavailable template</option>
            )}
            {templates.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </label>
        <a className="wf-text-link" href="#templates">
          Manage email templates <Icon name="arrowUp" />
        </a>
        {template ? (
          <div className="wf-email-preview">
            <span>EMAIL PREVIEW</span>
            <strong>{renderTemplate(template.subject)}</strong>
            <p>{renderTemplate(template.body)}</p>
          </div>
        ) : (
          <div className="wf-tip">
            Choose the email you want to send. You can save this draft and
            choose later.
          </div>
        )}
      </div>
    );
  }
  if (step.kind === "delay")
    return <DurationSettings step={step} change={change} />;
  if (step.kind === "schedule")
    return <ScheduleSettings step={step} change={change} />;
  if (step.kind !== "wait_reply") return null;
  const current =
    step.expected === "text" && outcome === "invalid" ? "success" : outcome;
  return (
    <div className="wf-fields">
      <div className="wf-tip amber">
        <Icon name="clock" />
        <p>
          Wait for a new DM from the same person and social account. Each
          outcome below has its own settings.
        </p>
      </div>
      <label>
        What are you waiting for?
        <select
          value={step.expected}
          onChange={(e) =>
            change({ ...step, expected: e.target.value as "email" | "text" })
          }
        >
          <option value="email">An email address</option>
          <option value="text">Any non-empty reply</option>
        </select>
      </label>
      <div className="wf-outcome-tabs" role="group" aria-label="Reply outcomes">
        {(
          [
            ["success", step.expected === "email" ? "Received" : "Reply"],
            ...(step.expected === "email" ? [["invalid", "Invalid"]] : []),
            ["timeout", "Timeout"],
          ] as [Outcome, string][]
        ).map(([key, label]) => (
          <button
            key={key}
            aria-pressed={current === key}
            onClick={() => onOutcome(key)}
          >
            {label}
          </button>
        ))}
      </div>
      {current === "success" && (
        <section className="wf-outcome green">
          <h3>
            <Icon name="check" />
            {step.expected === "email" ? "Email received" : "Reply received"}
          </h3>
          <p>
            {step.expected === "email"
              ? "Find one email address, even inside a sentence. Save it as contact.email and continue."
              : "Continue when a new non-empty message arrives."}
          </p>
          <span>
            Continue on the main path <Icon name="arrow" />
          </span>
        </section>
      )}
      {current === "invalid" && (
        <section className="wf-outcome amber">
          <h3>
            <Icon name="branch" />
            Not a valid email
          </h3>
          <label>
            Then
            <select
              value={step.invalidBehavior}
              onChange={(e) =>
                change({
                  ...step,
                  invalidBehavior: e.target.value as "retry" | "stop",
                })
              }
            >
              <option value="retry">Ask again and keep waiting</option>
              <option value="stop">End this run</option>
            </select>
          </label>
          {step.invalidBehavior === "retry" && (
            <>
              <label>
                Retry DM
                <textarea
                  rows={4}
                  value={step.retryMessage}
                  maxLength={4000}
                  onChange={(e) =>
                    change({ ...step, retryMessage: e.target.value })
                  }
                />
              </label>
              <label>
                Stop after this many invalid replies
                <input
                  type="number"
                  min={1}
                  max={10}
                  value={step.maxAttempts}
                  onChange={(e) =>
                    change({ ...step, maxAttempts: Number(e.target.value) })
                  }
                />
              </label>
              <p>
                No retry is sent after the final invalid reply. The timeout does
                not reset.
              </p>
            </>
          )}
        </section>
      )}
      {current === "timeout" && (
        <section className="wf-outcome neutral">
          <h3>
            <Icon name="clock" />
            No reply in time
          </h3>
          <label>
            Timeout (hours)
            <input
              type="number"
              min={1}
              max={720}
              value={step.timeoutHours}
              onChange={(e) =>
                change({ ...step, timeoutHours: Number(e.target.value) })
              }
            />
          </label>
          <label>
            When time runs out
            <select
              value={step.timeoutBehavior}
              onChange={(e) =>
                change({
                  ...step,
                  timeoutBehavior: e.target.value as "stop" | "dm" | "branch",
                })
              }
            >
              <option value="stop">End this run</option>
              <option value="dm">Send a final DM, then end</option>
              <option value="branch" disabled={depth >= 3}>
                Continue on a separate path
              </option>
            </select>
          </label>
          {step.timeoutBehavior === "dm" && (
            <label>
              Final DM
              <textarea
                rows={3}
                maxLength={4000}
                value={step.timeoutMessage}
                onChange={(e) =>
                  change({ ...step, timeoutMessage: e.target.value })
                }
              />
            </label>
          )}
          {step.timeoutBehavior === "branch" && (
            <>
              <p>
                This path replaces the main continuation when no reply arrives.
                Add a reminder, another wait, or any other step. It ends
                separately.
              </p>
              <button
                className="button button-secondary"
                onClick={onAddTimeout}
              >
                <Icon name="plus" /> Add timeout step
              </button>
            </>
          )}
          <p>
            Late replies cannot resume this expired wait. A later wait listens
            for a new message.
          </p>
        </section>
      )}
      <p className="wf-help">
        Reply window: {step.timeoutHours} hours.
        {step.expected === "email" &&
          " Email format is checked; ownership and deliverability are not verified."}
      </p>
    </div>
  );
}
function DurationSettings({
  step,
  change,
}: {
  step: Extract<Step, { kind: "delay" }>;
  change: (step: Step) => void;
}) {
  const [unit, setUnit] = useState(
    step.minutes && step.minutes % 1440 === 0
      ? 1440
      : step.minutes && step.minutes % 60 === 0
        ? 60
        : 1,
  );
  return (
    <div className="wf-fields">
      <div className="wf-tip amber">
        <Icon name="clock" />
        <p>Pause this session, then continue with the next step.</p>
      </div>
      <div className="wf-duration-input">
        <label>
          Wait for
          <input
            type="number"
            min={1 / unit}
            step="any"
            max={43200 / unit}
            value={step.minutes / unit}
            onChange={(e) =>
              change({ ...step, minutes: Number(e.target.value) * unit })
            }
          />
        </label>
        <label>
          Unit
          <select
            value={unit}
            onChange={(e) => {
              const next = Number(e.target.value);
              change({ ...step, minutes: (step.minutes / unit) * next });
              setUnit(next);
            }}
          >
            <option value={1}>Minutes</option>
            <option value={60}>Hours</option>
            <option value={1440}>Days</option>
          </select>
        </label>
      </div>
      <div className="wf-presets">
        {[
          [15, "15 min"],
          [60, "1 hour"],
          [1440, "1 day"],
        ].map(([minutes, label]) => (
          <button
            key={minutes}
            className={step.minutes === minutes ? "selected" : ""}
            onClick={() => {
              setUnit(
                Number(minutes) >= 1440 ? 1440 : Number(minutes) >= 60 ? 60 : 1,
              );
              change({ ...step, minutes: Number(minutes) });
            }}
          >
            {label}
          </button>
        ))}
      </div>
      <p className="wf-help">
        Starts when this step is reached. Maximum: 30 days.
      </p>
    </div>
  );
}
function ScheduleSettings({
  step,
  change,
}: {
  step: Extract<Step, { kind: "schedule" }>;
  change: (step: Step) => void;
}) {
  const [dateError, setDateError] = useState("");
  const localZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const zones = [
    ...new Set([
      step.timezone,
      localZone,
      "UTC",
      "Europe/London",
      "Europe/Berlin",
      "Europe/Moscow",
      "America/New_York",
      "America/Los_Angeles",
      "Asia/Dubai",
      "Asia/Singapore",
      "Asia/Tokyo",
      "Australia/Sydney",
    ]),
  ];
  return (
    <div className="wf-fields">
      <label>
        Resume at
        <select
          value={step.mode ?? "date"}
          onChange={(e) =>
            change({ ...step, mode: e.target.value as "date" | "next_time" })
          }
        >
          <option value="date">A specific date & time</option>
          <option value="next_time">The next occurrence of a time</option>
        </select>
      </label>
      {step.mode === "next_time" ? (
        <>
          <label>
            Time of day
            <input
              type="time"
              value={step.time ?? ""}
              onChange={(e) => change({ ...step, time: e.target.value })}
            />
          </label>
          <label>
            Timezone
            <select
              value={step.timezone}
              onChange={(e) => change({ ...step, timezone: e.target.value })}
            >
              {zones.map((zone) => (
                <option key={zone}>{zone}</option>
              ))}
            </select>
          </label>
          <div className="wf-tip amber">
            <Icon name="clock" />
            <p>
              Continue at the next {step.time || "chosen time"}: today if it
              hasn’t passed, otherwise the next day. This runs once per session.
            </p>
          </div>
          <p className="wf-help">
            If this time is skipped by daylight saving, use its next occurrence.
            During repeated hours, use the earliest occurrence still ahead.
          </p>
        </>
      ) : (
        <>
          <label>
            Date & time
            <input
              type="datetime-local"
              value={localDateValue(step.at)}
              onChange={(e) => {
                const at = localDateToInstant(e.target.value);
                setDateError(
                  e.target.value && !at
                    ? "That local time does not exist. Choose another time."
                    : "",
                );
                change({ ...step, at: at ?? "", timezone: localZone });
              }}
            />
          </label>
          {dateError && (
            <p className="wf-error" role="alert">
              {dateError}
            </p>
          )}
          <p className="wf-help">
            Input timezone: {localZone}. If this date has passed when the step
            is reached, continue immediately. During a repeated hour, use the
            earlier occurrence.
          </p>
        </>
      )}
      <div className="wf-variable">{formatSchedule(step)}</div>
    </div>
  );
}
