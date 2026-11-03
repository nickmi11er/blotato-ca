import { Icon } from "../../../shared/ui/Icon";
import { Section } from "../../../shared/ui/Section";
import { TagInput } from "../../../shared/ui/TagInput";
import type { Draft, UpdateDraft, MatchOperator } from "../model/types";
export function TriggerRules({
  draft,
  update,
}: {
  draft: Draft;
  update: UpdateDraft;
}) {
  return (
    <Section
      number="02"
      title="Set the spark"
      subtitle="Decide which comments start a conversation."
      icon="spark"
    >
      <div className="match-kind-grid">
        <button
          className={`match-kind ${draft.matchKind === "keywords" ? "selected" : ""}`}
          onClick={() => update({ matchKind: "keywords" })}
          aria-pressed={draft.matchKind === "keywords"}
        >
          <span className="match-kind-icon text-icon">Aa</span>
          <strong>Words & phrases</strong>
          <small>A word is all it takes</small>
          <span className="radio-dot" />
        </button>
        <button
          className={`match-kind ${draft.matchKind === "any" ? "selected" : ""}`}
          onClick={() => update({ matchKind: "any" })}
          aria-pressed={draft.matchKind === "any"}
        >
          <span className="match-kind-icon">
            <Icon name="comment" />
          </span>
          <strong>Any comment</strong>
          <small>Welcome every conversation</small>
          <span className="radio-dot" />
        </button>
        <button
          className="match-kind coming-soon"
          disabled
          title="Intent recognition is planned for a future version"
        >
          <span className="match-kind-icon">
            <Icon name="spark" />
          </span>
          <strong>
            By intent <span>SOON</span>
          </strong>
          <small>Understand the meaning</small>
        </button>
      </div>
      {draft.matchKind === "keywords" ? (
        <>
          <div className="matching-row">
            <label htmlFor="match-operator">
              Match type
              <select
                id="match-operator"
                value={draft.operator}
                onChange={(event) =>
                  update({
                    operator: event.target.value as MatchOperator,
                    logic: event.target.value === "exact" ? "any" : draft.logic,
                  })
                }
              >
                <option value="whole_word">Whole word or phrase</option>
                <option value="exact">Exact comment</option>
                <option value="contains">Contains text</option>
              </select>
            </label>
            <label htmlFor="match-logic">
              When there are multiple
              <select
                id="match-logic"
                disabled={draft.operator === "exact" && draft.logic === "any"}
                value={draft.logic}
                onChange={(event) =>
                  update({
                    logic: event.target.value as "any" | "all",
                  })
                }
              >
                <option value="any">Match any keyword</option>
                <option value="all" disabled={draft.operator === "exact"}>
                  Match every keyword
                </option>
              </select>
            </label>
          </div>
          <div className="field-heading">
            <label htmlFor="keywords-input">Your keywords</label>
            <span>Press Enter to add</span>
          </div>
          <TagInput
            id="keywords-input"
            label="Keywords"
            values={draft.keywords}
            onChange={(keywords) => update({ keywords })}
            caseSensitive={draft.caseSensitive}
          />
          <p className="helper keyword-helper">
            {draft.operator === "whole_word"
              ? "Matches the complete word, even inside a longer comment."
              : draft.operator === "exact"
                ? "The entire comment must match a keyword. Punctuation matters."
                : "Matches the text anywhere, including inside another word."}
          </p>
        </>
      ) : (
        <div className="any-comment-note">
          <Icon name="comment" />
          <div>
            <strong>Every comment is a new opportunity.</strong>
            <p>
              All comments in your selected scope will match. Your own comments
              are skipped.
            </p>
          </div>
        </div>
      )}
      <details className="advanced-options">
        <summary>
          <Icon name="settings" /> Fine-tune your trigger <span>Optional</span>
          <Icon name="chevron" />
        </summary>
        <div className="advanced-body">
          <div className="switch-row">
            <div>
              <strong>Case-sensitive matching</strong>
              <p>Treat “pricing” and “PRICING” differently.</p>
            </div>
            <label className="switch">
              <input
                type="checkbox"
                checked={draft.caseSensitive}
                onChange={(event) =>
                  update({ caseSensitive: event.target.checked })
                }
                aria-label="Case-sensitive matching"
              />
              <span />
            </label>
          </div>
          <label className="field-label" htmlFor="exclusions-input">
            Skip comments containing
          </label>
          <TagInput
            id="exclusions-input"
            label="Exclusions"
            values={draft.exclusions}
            onChange={(exclusions) => update({ exclusions })}
            exclusion
            caseSensitive={draft.caseSensitive}
          />
          <p className="helper">
            Exclusions always match whole words or phrases.
          </p>
        </div>
      </details>
    </Section>
  );
}
