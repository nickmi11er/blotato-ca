import { Icon } from "../../shared/ui/Icon";
import { PlatformBadge } from "./components/PlatformBadge";
import type { Draft, ScopeCatalog } from "./model/types";

export function AutomationList({
  draft,
  catalog,
  dirty,
  savedAt,
}: {
  draft: Draft;
  catalog: ScopeCatalog;
  dirty: boolean;
  savedAt: string | null;
}) {
  const accounts = catalog.accounts.filter((account) =>
    draft.accountIds.includes(account.id),
  );
  return (
    <main id="automations" tabIndex={-1} className="automation-list">
      <div className="automation-list-heading">
        <h1>
          Automations <span>1</span>
        </h1>
        <p>Your comment automations, all in one place.</p>
      </div>
      <a
        className="automation-list-item"
        href="#builder"
        aria-label={`Edit ${draft.name}`}
      >
        <span className="automation-symbol">
          <Icon name="bolt" />
        </span>
        <div className="automation-list-copy">
          <h2>{draft.name}</h2>
          <p>
            Comment trigger <span aria-hidden="true">→</span> Email capture
          </p>
          <span className="automation-list-status">
            {dirty
              ? "Unsaved changes"
              : savedAt
                ? "Saved on this device"
                : "Not saved yet"}
          </span>
        </div>
        <div className="automation-list-accounts">
          {accounts.map((account) => (
            <PlatformBadge key={account.id} account={account} small />
          ))}
        </div>
        <span className="draft-badge">Draft</span>
        <Icon name="arrow" className="automation-list-arrow" />
      </a>
    </main>
  );
}
