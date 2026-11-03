import { useEffect, useState } from "react";
import { AppLayout } from "./AppLayout";
import { useAutomationPage } from "./useAutomationPage";
import { AutomationList } from "../feature/automation/AutomationList";
import { AutomationBuilder } from "../feature/automation/AutomationBuilder";
import {
  AutomationDialogs,
  type EditorModal,
} from "../feature/automation/components/AutomationDialogs";
import {
  accounts,
  posts,
  initialComment,
} from "../feature/automation/fixtures";
import { defaultDraft } from "../feature/automation/model/defaultDraft";
import { useAutomationDraft } from "../feature/automation/model/useAutomationDraft";
import { Icon } from "../shared/ui/Icon";

const catalog = { accounts, posts };

export default function App() {
  const page = useAutomationPage();
  const controller = useAutomationDraft(defaultDraft, catalog);
  const [modal, setModal] = useState<EditorModal>(null);
  const [toast, setToast] = useState("");

  useEffect(() => {
    document.getElementById(page)?.focus();
  }, [page]);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(""), 4000);
    return () => window.clearTimeout(timer);
  }, [toast]);

  return (
    <>
      <a className="skip-link" href={`#${page}`}>
        Skip to content
      </a>
      <AppLayout
        onAccounts={() => setModal("accounts")}
        onActivity={() => setModal("activity")}
      >
        {page === "automations" ? (
          <AutomationList
            draft={controller.draft}
            catalog={catalog}
            dirty={controller.dirty}
            savedAt={controller.savedAt}
          />
        ) : (
          <AutomationBuilder
            controller={controller}
            catalog={catalog}
            initialComment={initialComment}
            setModal={setModal}
            save={() => setToast(controller.save())}
          />
        )}
      </AppLayout>
      <AutomationDialogs
        modal={modal}
        setModal={setModal}
        draft={controller.draft}
        update={controller.update}
        savedAt={controller.savedAt}
        setToast={setToast}
        catalog={catalog}
      />
      <div
        className={`toast ${toast ? "visible" : ""}`}
        role="status"
        aria-live="polite"
      >
        {toast && (
          <>
            <Icon name="info" />
            {toast}
          </>
        )}
      </div>
    </>
  );
}
