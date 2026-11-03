import { Icon } from "../../shared/ui/Icon";
import { TriggerScope } from "./components/TriggerScope";
import { TriggerRules } from "./components/TriggerRules";
import { RepeatBehavior } from "./components/RepeatBehavior";
import { TriggerPreview } from "./components/TriggerPreview";
import type { EditorModal } from "./components/AutomationDialogs";
import type { AutomationDraftController } from "./model/useAutomationDraft";
import type { ScopeCatalog, TestComment } from "./model/types";
export function AutomationBuilder({
  controller,
  catalog,
  initialComment,
  setModal,
  save,
}: {
  controller: AutomationDraftController;
  catalog: ScopeCatalog;
  initialComment: TestComment;
  setModal: (modal: EditorModal) => void;
  save: () => void;
}) {
  const { draft, update, toggleAccount, dirty, savedAt, errors } = controller;
  return (
    <main id="builder" tabIndex={-1}>
      <a className="back-to-automations" href="#automations">
        <Icon name="arrowLeft" /> Automations
      </a>
      <div className="editor-toolbar">
        <div className="automation-name">
          <span className="automation-symbol">
            <Icon name="bolt" />
          </span>
          <h1>
            <button
              onClick={() => setModal("rename")}
              title="Rename automation"
            >
              {draft.name}
              <Icon name="edit" />
            </button>
          </h1>
          <span className="draft-badge">Draft</span>
        </div>
        <div className="toolbar-actions">
          <span className="save-status">
            {dirty
              ? "Unsaved changes"
              : savedAt
                ? "Saved on this device"
                : "Not saved yet"}
          </span>
          <button className="button button-primary" onClick={save}>
            <Icon name="save" /> Save draft
          </button>
        </div>
      </div>

      <div className="builder-layout">
        <div className="editor-column">
          <TriggerScope
            draft={draft}
            update={update}
            catalog={catalog}
            toggleAccount={toggleAccount}
          />
          <TriggerRules draft={draft} update={update} />
          <RepeatBehavior draft={draft} update={update} />
          <section className="next-step-card">
            <span className="next-step-icon">
              <Icon name="arrow" />
            </span>
            <div>
              <span className="mini-label">NEXT, MAKE IT PERSONAL</span>
              <h3>Reply. Connect. Deliver.</h3>
              <p>Your four-step email capture flow is ready to preview.</p>
            </div>
            <button
              className="text-button"
              onClick={() => setModal("messages")}
            >
              Edit messages <Icon name="arrow" />
            </button>
          </section>
          <div className="editor-footer">
            <span>
              <Icon name="check" /> Your own comments are always skipped
            </span>
            <span>Built for every channel</span>
          </div>
          {errors.length > 0 && (
            <div className="validation-note" role="status">
              <Icon name="info" /> {errors[0]}
            </div>
          )}
        </div>
        <TriggerPreview
          draft={draft}
          catalog={catalog}
          initialComment={initialComment}
        />
      </div>
    </main>
  );
}
