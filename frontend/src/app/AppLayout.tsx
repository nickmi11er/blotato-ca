import type { ReactNode } from "react";
import blotatoLogo from "../../assets/blotato.webp";
import { Icon } from "../shared/ui/Icon";
export function AppLayout({
  children,
  onAccounts,
  onActivity,
}: {
  children: ReactNode;
  onAccounts: () => void;
  onActivity: () => void;
}) {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a
          className="brand"
          href="#automations"
          aria-label="Blotato automations"
        >
          <img src={blotatoLogo} alt="" />
          <span>BLOTATO</span>
          <span className="brand-dot" />
        </a>
        <div className="workspace-card">
          <span className="workspace-icon">
            n<span>✳</span>
          </span>
          <div>
            <strong>Northstar Studio</strong>
            <span>Your workspace</span>
          </div>
        </div>
        <span className="nav-label">WORKSPACE</span>
        <nav aria-label="Workspace">
          <a className="nav-item" href="#automations">
            <Icon name="grid" /> Overview
          </a>
          <a
            className="nav-item active"
            href="#automations"
            aria-current="page"
          >
            <Icon name="bolt" /> Automations{" "}
            <span className="nav-count">1</span>
          </a>
          <button className="nav-item" onClick={() => onAccounts()}>
            <Icon name="users" /> Social accounts{" "}
            <span className="connected-dot" />
          </button>
          <button className="nav-item" onClick={() => onActivity()}>
            <Icon name="activity" /> Activity
          </button>
        </nav>
      </aside>

      <div className="main-shell">
        {children}
        <footer className="page-footer">
          <span>Made for meaningful connections.</span>
          <span>
            Design preview <span>·</span> Drafts stay in this browser
          </span>
        </footer>
      </div>
    </div>
  );
}
