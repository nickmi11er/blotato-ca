import { useState } from "react";
import { Icon } from "../../shared/ui/Icon";
import { Modal } from "../../shared/ui/Modal";
import { allActions, type Step } from "../automation/model/workflow";
import { renderTemplate, type EmailTemplate } from "./model/templates";
import type { useTemplates } from "./model/useTemplates";
export function EmailTemplates({
  controller,
  steps,
  notify,
}: {
  controller: ReturnType<typeof useTemplates>;
  steps: Step[];
  notify: (message: string) => void;
}) {
  const [editing, setEditing] = useState<EmailTemplate | null>(null);
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState<EmailTemplate | null>(null);
  const referenced = new Set(
    allActions(steps)
      .filter((a) => a.kind === "email")
      .map((a) => a.templateId),
  );
  const open = (template: EmailTemplate) => {
    setEditing({ ...template });
    setError("");
  };
  return (
    <main id="templates" tabIndex={-1} className="wf-templates">
      <a href="#builder" className="back-to-automations">
        <Icon name="arrowLeft" /> Back to your flow
      </a>
      <div className="wf-templates-header">
        <div>
          <span className="mini-label">YOUR WORDS, READY TO GO</span>
          <h1>
            Email templates<span>.</span>
          </h1>
          <p>Write it once. Make every follow-up feel personal.</p>
        </div>
        <button
          className="button button-primary"
          onClick={() =>
            open({ id: crypto.randomUUID(), name: "", subject: "", body: "" })
          }
        >
          <Icon name="plus" /> New template
        </button>
      </div>
      <div className="wf-template-grid">
        {controller.templates.map((template) => (
          <article className="wf-template-card" key={template.id}>
            <div className="wf-template-card-top">
              <span className="wf-icon green">
                <Icon name="mail" />
              </span>
              <span
                className={
                  referenced.has(template.id)
                    ? "wf-template-used"
                    : "wf-template-tag"
                }
              >
                {referenced.has(template.id)
                  ? "Used in automations"
                  : "Email template"}
              </span>
            </div>
            <h2>{template.name}</h2>
            <div className="wf-template-paper">
              <strong>{renderTemplate(template.subject)}</strong>
              <p>{renderTemplate(template.body)}</p>
              <span />
            </div>
            <div className="wf-template-card-footer">
              <button className="text-button" onClick={() => open(template)}>
                Edit template <Icon name="arrow" />
              </button>
              <button
                className="wf-icon-button"
                disabled={referenced.has(template.id)}
                title={
                  referenced.has(template.id)
                    ? "Remove this template from all automations and save their drafts before deleting it"
                    : "Delete template"
                }
                aria-label={`Delete ${template.name}`}
                onClick={() => setDeleting(template)}
              >
                <Icon name="trash" />
              </button>
            </div>
          </article>
        ))}
        <button
          className="wf-template-new"
          onClick={() =>
            open({ id: crypto.randomUUID(), name: "", subject: "", body: "" })
          }
        >
          <span>
            <Icon name="plus" />
          </span>
          <strong>A new conversation starter</strong>
          <small>Create an email template</small>
        </button>
      </div>
      <p className="wf-template-footnote">
        <Icon name="info" /> Templates are stored on this device. Emails shown
        here are previews, not deliveries.
      </p>
      {editing && (
        <Modal
          title={
            controller.templates.some((t) => t.id === editing.id)
              ? "Make it sound like you."
              : "A little more personal."
          }
          eyebrow="EMAIL TEMPLATE"
          description="Create a reusable email for any step in your flow."
          close={() => setEditing(null)}
          className="wf-template-modal"
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const issue = controller.save(editing);
              if (issue) setError(issue);
              else {
                setEditing(null);
                notify("Email template saved on this device.");
              }
            }}
          >
            <div className="wf-template-editor">
              <div className="wf-fields">
                <label>
                  Template name
                  <input
                    value={editing.name}
                    required
                    maxLength={100}
                    placeholder="e.g. Pricing introduction"
                    onChange={(e) =>
                      setEditing({ ...editing, name: e.target.value })
                    }
                  />
                </label>
                <label>
                  Subject
                  <input
                    value={editing.subject}
                    required
                    maxLength={200}
                    placeholder="Your next chapter starts here"
                    onChange={(e) =>
                      setEditing({ ...editing, subject: e.target.value })
                    }
                  />
                </label>
                <label>
                  Email body
                  <textarea
                    rows={11}
                    value={editing.body}
                    required
                    maxLength={10000}
                    placeholder="Hey {{first_name}},…"
                    onChange={(e) =>
                      setEditing({ ...editing, body: e.target.value })
                    }
                  />
                </label>
                <p className="wf-help">
                  Plain text with links. Personalize with {"{{first_name}}"} and{" "}
                  {"{{email}}"}. Changing a template updates all local draft
                  steps using it.
                </p>
              </div>
              <div className="wf-template-live">
                <span className="mini-label">LIVE PREVIEW</span>
                <div className="wf-mail-envelope">
                  <div>
                    <span className="wf-avatar">N</span>
                    <span>
                      <strong>Northstar Studio</strong>
                      <small>To Jordan · jordan@example.com</small>
                    </span>
                  </div>
                  <h3>
                    {renderTemplate(editing.subject) || "Your subject line"}
                  </h3>
                  <p>
                    {renderTemplate(editing.body) ||
                      "Your email will appear here as you write."}
                  </p>
                </div>
                <small>Sample contact · Jordan</small>
              </div>
            </div>
            {error && (
              <p className="wf-error" role="alert">
                {error}
              </p>
            )}
            <div className="wf-modal-footer">
              <button
                type="button"
                className="button button-secondary"
                onClick={() => setEditing(null)}
              >
                Cancel
              </button>
              <button type="submit" className="button button-primary">
                <Icon name="check" /> Save template
              </button>
            </div>
          </form>
        </Modal>
      )}
      {deleting && (
        <Modal
          title="Delete this template?"
          eyebrow="EMAIL TEMPLATES"
          description={`“${deleting.name}” will be removed from this browser.`}
          close={() => setDeleting(null)}
        >
          <div className="wf-modal-footer">
            <button
              className="button button-secondary"
              onClick={() => setDeleting(null)}
            >
              Keep template
            </button>
            <button
              className="button button-primary"
              onClick={() => {
                const issue = controller.remove(deleting.id);
                if (issue) notify(issue);
                else notify("Template deleted.");
                setDeleting(null);
              }}
            >
              Delete template
            </button>
          </div>
        </Modal>
      )}
    </main>
  );
}
