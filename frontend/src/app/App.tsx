import { useEffect, useState } from "react";
import { useTemplates } from "../feature/email-templates/model/useTemplates";
import { EmailTemplates } from "../feature/email-templates/EmailTemplates";
import { AppLayout } from "./AppLayout";
import { useAutomationPage } from "./useAutomationPage";
import { AutomationList } from "../feature/automation/AutomationList";
import { AutomationBuilder } from "../feature/automation/AutomationBuilder";
import {
  AutomationDialogs,
  type EditorModal,
} from "../feature/automation/components/AutomationDialogs";
import { accounts, posts } from "../feature/automation/fixtures";
import { defaultDraft } from "../feature/automation/model/defaultDraft";
import { useAutomations } from "../feature/automation/model/useAutomations";
import { Icon } from "../shared/ui/Icon";

const catalog = { accounts, posts };

export default function App() {
  const requestedPage = useAutomationPage();
  const templates = useTemplates();
  const automations = useAutomations(
    defaultDraft,
    catalog,
    templates.templates,
  );
  const { controller } = automations;
  const page =
    requestedPage === "builder" && !controller ? "automations" : requestedPage;
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
      <AppLayout page={page} automationCount={automations.automations.length}>
        {automations.storageError && (
          <p className="wf-error" role="alert">
            {automations.storageError}
          </p>
        )}
        {page === "templates" ? (
          <EmailTemplates
            controller={templates}
            steps={automations.templateSteps}
            notify={setToast}
          />
        ) : page === "automations" ? (
          <AutomationList
            automations={automations.automations}
            catalog={catalog}
            select={automations.select}
            remove={(id) => {
              const error = automations.remove(id);
              if (!error) setToast("Automation deleted.");
              return error;
            }}
            create={() => {
              const error = automations.create();
              if (error) setToast(error);
              else {
                setToast("");
                window.location.hash = "#builder";
              }
            }}
          />
        ) : controller ? (
          <AutomationBuilder
            key={automations.activeId}
            controller={controller}
            catalog={catalog}
            templates={templates.templates}
            setModal={setModal}
            save={() => setToast(controller.save())}
          />
        ) : null}
      </AppLayout>
      {controller && (
        <AutomationDialogs
          modal={modal}
          setModal={setModal}
          draft={controller.draft}
          update={controller.update}
        />
      )}
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
