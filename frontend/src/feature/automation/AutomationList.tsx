import { useEffect, useRef, useState } from "react";
import { Modal } from "../../shared/ui/Modal";
import { Icon } from "../../shared/ui/Icon";
import { PlatformBadge } from "./components/PlatformBadge";
import type { ScopeCatalog } from "./model/types";
import type { AutomationSummary } from "./model/useAutomations";

export function AutomationList({
  automations,
  catalog,
  select,
  create,
  remove,
}: {
  automations: AutomationSummary[];
  catalog: ScopeCatalog;
  select: (id: string) => void;
  create: () => void;
  remove: (id: string) => string | null;
}) {
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<AutomationSummary | null>(null);
  const [error, setError] = useState("");
  const menuRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!openMenu) return;
    const outside = (event: Event) => {
      if (
        event.target instanceof Node &&
        !menuRef.current?.contains(event.target)
      )
        setOpenMenu(null);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      menuRef.current?.querySelector("button")?.focus();
      setOpenMenu(null);
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("focusin", outside);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("focusin", outside);
      document.removeEventListener("keydown", escape);
    };
  }, [openMenu]);
  return (
    <main id="automations" tabIndex={-1} className="automation-list">
      <div className="automation-list-heading">
        <div>
          <h1>
            Automations <span>{automations.length}</span>
          </h1>
          <p>Your comment automations, all in one place.</p>
        </div>
        <button className="button button-primary" onClick={create}>
          <Icon name="plus" /> New automation
        </button>
      </div>
      <div className="automation-list-items">
        {automations.map(({ id, draft, dirty, savedAt }) => (
          <div className="automation-card" key={id}>
            <a
              className="automation-list-item"
              href="#builder"
              aria-label={`Edit ${draft.name}`}
              onClick={() => select(id)}
            >
              <div className="automation-card-top">
                <span className="automation-symbol">
                  <Icon name="bolt" />
                </span>
                <span className="draft-badge">Draft</span>
              </div>
              <div className="automation-list-copy">
                <h2 title={draft.name}>{draft.name}</h2>
                <p className="automation-card-trigger">
                  <Icon name="comment" />
                  <span>
                    {draft.matchKind === "any"
                      ? "Any comment"
                      : draft.keywords.length
                        ? `${draft.operator === "exact" ? "Exact match" : draft.operator === "contains" ? "Contains" : "Matches words"} · ${draft.keywords.map((keyword) => `“${keyword}”`).join(draft.logic === "all" ? " and " : " or ")}`
                        : "Choose your trigger"}
                  </span>
                </p>
              </div>
              <div className="automation-card-meta">
                <span>
                  <Icon name="branch" />
                  {draft.steps.length}{" "}
                  {draft.steps.length === 1 ? "step" : "steps"}
                </span>
                <span>
                  {draft.postScope === "all"
                    ? "All posts"
                    : `${draft.postIds.length} selected ${draft.postIds.length === 1 ? "post" : "posts"}`}
                </span>
              </div>
              <div className="automation-card-footer">
                <div className="automation-list-accounts">
                  {catalog.accounts
                    .filter((account) => draft.accountIds.includes(account.id))
                    .map((account) => (
                      <span
                        key={account.id}
                        title={`${account.name} · ${account.handle}`}
                      >
                        <PlatformBadge account={account} small />
                      </span>
                    ))}
                  {!draft.accountIds.length && (
                    <span className="automation-no-accounts">
                      No accounts yet
                    </span>
                  )}
                </div>
                <span
                  className={`automation-list-status ${dirty ? "is-dirty" : ""}`}
                >
                  {dirty
                    ? "Unsaved changes"
                    : savedAt
                      ? "Saved on this device"
                      : "Not saved yet"}
                </span>
                <Icon name="arrow" className="automation-list-arrow" />
              </div>
            </a>
            <div
              className="automation-card-menu"
              ref={openMenu === id ? menuRef : null}
            >
              <button
                className="automation-card-menu-toggle"
                aria-label={`Options for ${draft.name}`}
                aria-expanded={openMenu === id}
                onClick={() => setOpenMenu(openMenu === id ? null : id)}
              >
                <Icon name="more" />
              </button>
              {openMenu === id && (
                <div className="automation-card-menu-items">
                  <button
                    onClick={() => {
                      setDeleting({ id, draft, dirty, savedAt });
                      setError("");
                      setOpenMenu(null);
                    }}
                  >
                    <Icon name="trash" /> Delete automation
                  </button>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
      {!automations.length && (
        <div className="automation-list-empty">
          <span className="automation-symbol">
            <Icon name="bolt" />
          </span>
          <h2>Your next conversation starts here.</h2>
          <p>Create an automation to turn a comment into a conversation.</p>
          <button className="button button-primary" onClick={create}>
            <Icon name="plus" /> Create your first automation
          </button>
        </div>
      )}
      {deleting && (
        <Modal
          title="Delete this automation?"
          eyebrow="DELETE AUTOMATION"
          description={`“${deleting.draft.name}” will be removed from this browser. This cannot be undone.`}
          close={() => setDeleting(null)}
        >
          {deleting.dirty && (
            <p className="wf-error">
              This automation has unsaved changes. They will also be deleted.
            </p>
          )}
          {error && (
            <p className="wf-error" role="alert">
              {error}
            </p>
          )}
          <div className="wf-modal-footer">
            <button
              autoFocus
              className="button button-secondary"
              onClick={() => setDeleting(null)}
            >
              Keep automation
            </button>
            <button
              className="button button-danger"
              onClick={() => {
                const issue = remove(deleting.id);
                if (issue) setError(issue);
                else setDeleting(null);
              }}
            >
              <Icon name="trash" /> Delete automation
            </button>
          </div>
        </Modal>
      )}
    </main>
  );
}
