import { Section } from "../../../shared/ui/Section";
import type { Draft, UpdateDraft, Reentry } from "../model/types";
export function RepeatBehavior({
  draft,
  update,
}: {
  draft: Draft;
  update: UpdateDraft;
}) {
  return (
    <Section
      number="03"
      title="Keep it thoughtful"
      subtitle="Choose when someone can start again."
      icon="users"
    >
      <label className="field-label" htmlFor="reentry">
        Repeat behavior
      </label>
      <select
        id="reentry"
        value={draft.reentry}
        onChange={(event) => update({ reentry: event.target.value as Reentry })}
      >
        <option value="skip_active">
          Only when they have no active conversation
        </option>
        <option value="once">Once per person for this automation</option>
        <option value="every">On every matching comment</option>
      </select>
      <p className="helper">
        {draft.reentry === "skip_active"
          ? "Give each conversation room to breathe before starting a new one."
          : draft.reentry === "once"
            ? "A person can enter this automation once per social account, across your selected posts."
            : "Each new matching comment can start a separate conversation."}
      </p>
    </Section>
  );
}
