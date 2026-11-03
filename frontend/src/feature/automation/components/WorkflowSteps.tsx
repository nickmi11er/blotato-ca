import { Icon } from "../../../shared/ui/Icon";
import type { Messages } from "../model/types";
export function WorkflowSteps({
  messages,
  compact = false,
}: {
  messages: Messages;
  compact?: boolean;
}) {
  const steps = [
    ["comment", "Reply to the comment", messages.reply, "pink"],
    [
      "mail",
      "Ask for their email",
      compact ? "Start a private conversation" : messages.question,
      "violet",
    ],
    [
      "clock",
      "Wait for a reply",
      "Continue when an email is received",
      "amber",
    ],
    [
      "link",
      "Send the link",
      compact ? "Deliver your pricing page" : messages.link,
      "green",
    ],
  ] as const;
  return (
    <div className="workflow-steps">
      {steps.map(([icon, title, subtitle, color], index) => (
        <div className="workflow-step" key={title}>
          <span className={`step-icon ${color}`}>
            <Icon name={icon} />
          </span>
          <div>
            <strong>{title}</strong>
            <p>{subtitle}</p>
          </div>
          <span className="step-order">0{index + 1}</span>
        </div>
      ))}
    </div>
  );
}
