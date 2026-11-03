import { useState, type FormEvent } from "react";
import { Icon } from "../../../shared/ui/Icon";
import type { Messages } from "../model/types";
import { validateMessages } from "../model/validation";
export function MessageEditor({
  messages,
  onSave,
}: {
  messages: Messages;
  onSave: (messages: Messages) => void;
}) {
  const [value, setValue] = useState(messages);
  const [error, setError] = useState("");
  function submit(event: FormEvent) {
    event.preventDefault();
    const errors = validateMessages(value);
    if (errors.length) {
      setError(errors[0]);
      return;
    }
    onSave(value);
  }
  return (
    <form className="modal-form" onSubmit={submit}>
      {(
        [
          ["reply", "Public reply", 2],
          ["question", "DM asking for their email", 3],
          ["linkText", "Follow-up DM", 2],
        ] as const
      ).map(([key, label, rows]) => (
        <label key={key}>
          {label}
          <textarea
            value={value[key]}
            onChange={(event) =>
              setValue({ ...value, [key]: event.target.value })
            }
            rows={rows}
            required
            maxLength={1000}
          />
        </label>
      ))}
      <label>
        Destination link
        <input
          value={value.link}
          onChange={(event) => setValue({ ...value, link: event.target.value })}
          type="url"
          required
          maxLength={1000}
        />
      </label>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <button className="button button-primary" type="submit">
        Save messages <Icon name="check" />
      </button>
    </form>
  );
}
