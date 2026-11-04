import { Icon, type IconName } from "../../../shared/ui/Icon";
import { Modal } from "../../../shared/ui/Modal";
import type { StepKind } from "../model/workflow";
export const stepOptions: {
  kind: StepKind;
  icon: IconName;
  label: string;
  description: string;
  group: string;
}[] = [
  {
    kind: "reply",
    icon: "comment",
    label: "Reply to comment",
    description: "Respond publicly to the original comment",
    group: "Take an action",
  },
  {
    kind: "dm",
    icon: "send",
    label: "Send a DM",
    description: "Use the same social account and conversation",
    group: "Take an action",
  },
  {
    kind: "email",
    icon: "mail",
    label: "Send an email",
    description: "Choose a reusable email template",
    group: "Take an action",
  },
  {
    kind: "wait_reply",
    icon: "comment",
    label: "Wait for a reply",
    description: "Collect an email or wait for any message",
    group: "Pause & resume",
  },
  {
    kind: "delay",
    icon: "clock",
    label: "Wait for a duration",
    description: "Continue after minutes, hours or days",
    group: "Pause & resume",
  },
  {
    kind: "schedule",
    icon: "calendar",
    label: "Wait until a date & time",
    description: "Resume at a specific moment",
    group: "Pause & resume",
  },
  {
    kind: "parallel",
    icon: "parallel",
    label: "Run together",
    description: "Perform multiple actions at the same time",
    group: "Organize your flow",
  },
];
export function StepPalette({
  add,
  close,
  actionsOnly = false,
}: {
  add: (kind: StepKind) => void;
  close: () => void;
  actionsOnly?: boolean;
}) {
  return (
    <Modal
      title={actionsOnly ? "Add an action" : "What happens next?"}
      eyebrow="BUILD YOUR FLOW"
      description={
        actionsOnly
          ? "These actions run together. The flow continues when all finish."
          : "Make a move, wait for the right moment, or bring actions together."
      }
      close={close}
    >
      <div className="wf-palette">
        {["Take an action", "Pause & resume", "Organize your flow"]
          .filter((group) => !actionsOnly || group === "Take an action")
          .map((group) => (
            <section key={group}>
              <h3>{group}</h3>
              {stepOptions
                .filter((option) => option.group === group)
                .map((option) => (
                  <button
                    aria-label={option.label}
                    key={option.kind}
                    onClick={() => add(option.kind)}
                  >
                    <span
                      className={`wf-icon ${group === "Pause & resume" ? "amber" : ""}`}
                    >
                      <Icon name={option.icon} />
                    </span>
                    <span>
                      <strong>{option.label}</strong>
                      <small>{option.description}</small>
                    </span>
                    <Icon name="plus" />
                  </button>
                ))}
            </section>
          ))}
      </div>
    </Modal>
  );
}
