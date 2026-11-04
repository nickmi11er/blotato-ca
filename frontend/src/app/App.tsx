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
import {
  automationAPI,
  loadCatalog,
  type AutomationRecord,
} from "../feature/automation/api/automations";
import { errorMessage, request } from "../shared/api/client";
import type { ScopeCatalog } from "../feature/automation/model/types";
import type { EmailTemplate } from "../feature/email-templates/model/templates";
import { defaultDraft } from "../feature/automation/model/defaultDraft";
import { useAutomations } from "../feature/automation/model/useAutomations";
import { Icon } from "../shared/ui/Icon";

interface WorkspaceData {
  catalog: ScopeCatalog;
  automations: AutomationRecord[];
  templates: EmailTemplate[];
}
export default function App() {
  const [data, setData] = useState<WorkspaceData | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    Promise.all([
      loadCatalog(controller.signal),
      automationAPI.list(controller.signal),
      request<EmailTemplate[]>("templates", { signal: controller.signal }),
    ])
      .then(([catalog, automations, templates]) => {
        if (!controller.signal.aborted) {
          setData({ catalog, automations, templates });
          setError("");
        }
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(errorMessage(e));
      });
    return () => controller.abort();
  }, [attempt]);
  if (!data)
    return (
      <AppLayout page="automations" automationCount={0}>
        <main className="connection-state" aria-busy={!error}>
          <span className="automation-symbol">
            <Icon name={error ? "info" : "bolt"} />
          </span>
          <h1>
            {error ? "Couldn’t load your workspace" : "Loading your workspace…"}
          </h1>
          <p role={error ? "alert" : "status"}>
            {error || "Getting your automations, accounts and templates."}
          </p>
          {error && (
            <button
              className="button button-primary"
              onClick={() => {
                setError("");
                setAttempt((n) => n + 1);
              }}
            >
              Try again
            </button>
          )}
        </main>
      </AppLayout>
    );
  return <Workspace data={data} />;
}

function Workspace({ data }: { data: WorkspaceData }) {
  const catalog = data.catalog;
  const requestedPage = useAutomationPage();
  const templates = useTemplates(data.templates);
  const automations = useAutomations(
    defaultDraft,
    catalog,
    templates.templates,
    data.automations,
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
            busy={automations.busy}
            remove={async (id) => {
              const error = await automations.remove(id);
              if (!error) setToast("Automation deleted.");
              return error;
            }}
            create={async () => {
              const error = await automations.create();
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
            templates={automations.templates}
            setModal={setModal}
            save={async () => setToast(await controller.save())}
            publish={async () => setToast(await controller.publish())}
            disable={async () => setToast(await controller.disable())}
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
