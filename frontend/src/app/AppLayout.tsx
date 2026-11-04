import type { ReactNode } from "react";
import blotatoLogo from "../../assets/blotato.webp";
import { Icon } from "../shared/ui/Icon";
export function AppLayout({
  children,
  page,
  automationCount,
}: {
  children: ReactNode;
  page: string;
  automationCount: number;
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
          <a
            className={`nav-item ${page !== "templates" ? "active" : ""}`}
            href="#automations"
            aria-current={page !== "templates" ? "page" : undefined}
          >
            <Icon name="bolt" /> Automations{" "}
            <span className="nav-count">{automationCount}</span>
          </a>
          <a
            className={`nav-item ${page === "templates" ? "active" : ""}`}
            aria-current={page === "templates" ? "page" : undefined}
            href="#templates"
          >
            <Icon name="mail" /> Email templates
          </a>
        </nav>
      </aside>

      <div className="main-shell">
        {children}
        <footer className="page-footer">
          <span>Made for meaningful connections.</span>
          <span>
            Connected workspace <span>·</span> Saved to your account
          </span>
        </footer>
      </div>
    </div>
  );
}
