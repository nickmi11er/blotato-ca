import { useRef, useState } from "react";
import { Icon } from "./Icon";
export function TagInput({
  values,
  onChange,
  id,
  label,
  exclusion = false,
  caseSensitive = false,
}: {
  values: string[];
  onChange: (values: string[]) => void;
  id: string;
  label: string;
  exclusion?: boolean;
  caseSensitive?: boolean;
}) {
  const [text, setText] = useState("");
  const ref = useRef<HTMLInputElement>(null);
  function add() {
    const value = text.trim().normalize("NFC");
    if (!value || values.length >= 30) return;
    const comparable = (entry: string) =>
      caseSensitive ? entry : entry.toLowerCase();
    if (!values.some((entry) => comparable(entry) === comparable(value)))
      onChange([...values, value]);
    setText("");
  }
  return (
    <div className={`tag-input ${exclusion ? "exclusion-input" : ""}`}>
      {values.map((value) => (
        <span
          className={`keyword-tag ${exclusion ? "exclusion" : ""}`}
          key={value}
        >
          {value}
          <button
            type="button"
            onClick={() => onChange(values.filter((entry) => entry !== value))}
            aria-label={`Remove ${value}`}
          >
            <Icon name="close" />
          </button>
        </span>
      ))}
      <input
        ref={ref}
        id={id}
        aria-label={label}
        value={text}
        onChange={(event) => setText(event.target.value)}
        onBlur={add}
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.nativeEvent.isComposing) {
            event.preventDefault();
            add();
          }
        }}
        placeholder={
          values.length
            ? "Add another…"
            : exclusion
              ? "Add an exclusion…"
              : "Type a word or phrase…"
        }
        maxLength={120}
        autoComplete="off"
      />
      <button
        className="tag-add"
        type="button"
        onClick={() => {
          add();
          ref.current?.focus();
        }}
        aria-label={`Add ${label.toLowerCase()}`}
        disabled={values.length >= 30}
      >
        <Icon name="plus" />
      </button>
    </div>
  );
}
