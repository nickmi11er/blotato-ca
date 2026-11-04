import { useState, useRef, useEffect } from "react";
import { Icon } from "../../shared/ui/Icon";
import { TriggerScope } from "./components/TriggerScope";
import { TriggerRules } from "./components/TriggerRules";
import { RepeatBehavior } from "./components/RepeatBehavior";
import type { EditorModal } from "./components/AutomationDialogs";
import type { AutomationDraftController } from "./model/useAutomations";
import type { ScopeCatalog } from "./model/types";
import type { EmailTemplate } from "../email-templates/model/templates";
import {
  createStep,
  stepTitle,
  type Step,
  type StepKind,
  walkSteps,
} from "./model/workflow";
import { validateWorkflow } from "./model/workflowValidation";
import { StepPalette } from "./workflow/StepPalette";
import { StepSettings } from "./workflow/StepSettings";
import {
  locateStep,
  replaceSequence,
  replaceStep,
  cloneStep,
  moveStep,
} from "./model/workflowEditing";
import type { Simulation } from "./model/simulation";
import {
  FlowCanvas,
  type Outcome,
  type AddPosition,
  type StepOperation,
} from "./workflow/FlowCanvas";
import { FlowTest } from "./workflow/FlowTest";

export function AutomationBuilder({
  controller,
  catalog,
  templates,
  setModal,
  save,
}: {
  controller: AutomationDraftController;
  catalog: ScopeCatalog;
  templates: readonly EmailTemplate[];
  setModal: (modal: EditorModal) => void;
  save: () => void;
}) {
  const { draft, update, toggleAccount, dirty, savedAt, errors } = controller;
  const [selected, setSelected] = useState("trigger");
  const [outcome, setOutcome] = useState<Outcome>("success");
  const inspectorRef = useRef<HTMLElement>(null);
  const canvasRef = useRef<HTMLElement>(null);
  const [adding, setAdding] = useState<AddPosition | null>(null);
  const [testing, setTesting] = useState(false);
  const [run, setRun] = useState<Simulation | null>(null);
  useEffect(() => {
    inspectorRef.current?.scrollTo?.({ top: 0 });
    if (testing && window.innerWidth <= 720)
      inspectorRef.current?.scrollIntoView?.({ block: "start" });
  }, [testing]);
  const [notice, setNotice] = useState("");
  const [undo, setUndo] = useState<{
    steps: Step[];
    selected: string;
    label: string;
  } | null>(null);
  const [showIssues, setShowIssues] = useState(false);
  const location = locateStep(draft.steps, selected);
  const active = location?.step;
  const issues = validateWorkflow(draft.steps, templates);
  const select = (id: string, nextOutcome: Outcome = "success") => {
    setSelected(id);
    setOutcome(nextOutcome);
    if (!testing) {
      inspectorRef.current?.scrollTo?.({ top: 0 });
      if (window.innerWidth <= 720)
        inspectorRef.current?.scrollIntoView?.({ block: "start" });
    }
  };
  function commit(steps: Step[], label: string, nextSelected = selected) {
    setUndo({ steps: draft.steps, selected, label });
    update({ steps });
    setSelected(nextSelected);
    setNotice(label);
  }
  function change(step: Step) {
    setUndo(null);
    update({ steps: replaceStep(draft.steps, step) });
  }
  function add(kind: StepKind) {
    if (!adding) return;
    const parent = adding.parent
      ? locateStep(draft.steps, adding.parent)?.step
      : undefined;
    const siblings =
      parent?.kind === "parallel"
        ? parent.actions
        : parent?.kind === "wait_reply"
          ? (parent.timeoutSteps ?? [])
          : draft.steps;
    if (
      siblings.length >= (adding.actionsOnly ? 8 : 40) ||
      walkSteps(draft.steps).length >= 160
    )
      return;
    const step = createStep(kind);
    const next = [...siblings];
    next.splice(adding.index, 0, step);
    commit(
      replaceSequence(draft.steps, adding.parent, next),
      "Step added",
      step.id,
    );
    setAdding(null);
    setShowIssues(false);
    select(step.id);
  }
  function operate(id: string, operation: StepOperation) {
    const entry = locateStep(draft.steps, id);
    if (!entry) return;
    if (operation === "up" || operation === "down") {
      const next = moveStep(
        draft.steps,
        id,
        entry.index + (operation === "up" ? -1 : 1),
      );
      if (next !== draft.steps) commit(next, `Step moved ${operation}`, id);
    } else if (operation === "remove") {
      const siblings = entry.siblings.filter((s) => s.id !== id);
      commit(
        replaceSequence(draft.steps, entry.parent, siblings),
        `${stepTitle(entry.step)} deleted`,
        siblings[Math.min(entry.index, siblings.length - 1)]?.id ??
          entry.parent ??
          "trigger",
      );
    } else {
      if (
        entry.siblings.length >= (entry.actionsOnly ? 8 : 40) ||
        walkSteps(draft.steps).length + walkSteps([entry.step]).length > 160
      )
        return;
      const copy = cloneStep(entry.step);
      const siblings = [...entry.siblings];
      siblings.splice(entry.index + 1, 0, copy);
      const next = replaceSequence(draft.steps, entry.parent, siblings);
      // Structural decoder also enforces nesting and unique IDs before persistence.
      commit(next, "Step duplicated", copy.id);
    }
  }
  function undoLast() {
    if (undo) {
      update({ steps: undo.steps });
      setSelected(undo.selected);
      setNotice(`${undo.label} — undone`);
      setUndo(null);
    }
  }
  const index = location?.index ?? -1;

  return (
    <main id="builder" tabIndex={-1} className="wf-builder">
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
          <button
            className="button button-secondary"
            onClick={() => {
              setTesting(!testing);
              setRun(null);
              setShowIssues(true);
            }}
          >
            <Icon name={testing ? "close" : "play"} />{" "}
            {testing ? "Exit test" : "Test flow"}
          </button>
          <button className="button button-primary" onClick={save}>
            <Icon name="save" /> Save draft
          </button>
        </div>
      </div>
      <div className="wf-workspace">
        <FlowCanvas
          draft={draft}
          selected={selected}
          templates={templates}
          issues={issues}
          select={select}
          onAdd={setAdding}
          canvasRef={canvasRef}
          operate={operate}
          testing={testing}
          run={run}
        />
        <aside
          ref={inspectorRef}
          className="wf-inspector"
          aria-label="Step settings"
        >
          {testing ? (
            <FlowTest
              draft={draft}
              templates={templates}
              catalog={catalog}
              close={() => {
                setTesting(false);
                setRun(null);
              }}
              run={run}
              setRun={setRun}
            />
          ) : (
            <>
              <div className="wf-inspector-head">
                <button
                  className="wf-mobile-back text-button"
                  onClick={() =>
                    canvasRef.current?.scrollIntoView({ block: "start" })
                  }
                >
                  <Icon name="arrowLeft" /> Back to flow
                </button>
                <span className="mini-label">
                  {selected === "trigger" || selected === "rules"
                    ? "TRIGGER SETTINGS"
                    : "STEP SETTINGS"}
                </span>
                <h2>
                  {selected === "trigger"
                    ? "Start a conversation"
                    : selected === "rules"
                      ? "Find the right comments"
                      : active
                        ? stepTitle(active)
                        : "Make it your own"}
                </h2>
                <p>
                  {active
                    ? "Shape what happens at this point in your flow."
                    : "Choose a step on the canvas to configure it."}
                </p>
              </div>
              <div className="wf-inspector-body">
                {selected === "trigger" ? (
                  <>
                    <TriggerScope
                      draft={draft}
                      update={update}
                      catalog={catalog}
                      toggleAccount={toggleAccount}
                    />
                    <RepeatBehavior draft={draft} update={update} />
                  </>
                ) : selected === "rules" ? (
                  <TriggerRules draft={draft} update={update} />
                ) : active ? (
                  <StepSettings
                    key={active.id}
                    step={active}
                    change={change}
                    templates={templates}
                    outcome={outcome}
                    onOutcome={setOutcome}
                    depth={location?.depth ?? 0}
                    onAddTimeout={() =>
                      setAdding({
                        index:
                          active.kind === "wait_reply"
                            ? (active.timeoutSteps?.length ?? 0)
                            : 0,
                        parent: active.id,
                      })
                    }
                  />
                ) : (
                  <div className="wf-tip">
                    Select a step or add your next action.
                  </div>
                )}
                {showIssues &&
                  issues
                    .filter((issue) => issue.stepId === selected)
                    .map((issue, i) => (
                      <p className="wf-error" role="status" key={i}>
                        <Icon name="info" /> {issue.message}
                      </p>
                    ))}
              </div>
              {active && (
                <div className="wf-inspector-tools">
                  <div>
                    {index >= 0 && (
                      <>
                        <button
                          disabled={index === 0}
                          aria-label="Move step up"
                          title="Move up"
                          onClick={() => operate(selected, "up")}
                        >
                          <Icon name="up" />
                        </button>
                        <button
                          disabled={
                            index === (location?.siblings.length ?? 0) - 1
                          }
                          aria-label="Move step down"
                          title="Move down"
                          onClick={() => operate(selected, "down")}
                        >
                          <Icon name="down" />
                        </button>
                      </>
                    )}
                  </div>
                  <button
                    className="wf-delete"
                    onClick={() => operate(selected, "remove")}
                  >
                    <Icon name="trash" /> Remove{" "}
                    {location?.actionsOnly ? "action" : "step"}
                  </button>
                </div>
              )}
              <div className="wf-local-note">
                <Icon name="save" />
                <span>
                  Draft only. Saved in this browser.
                  <br />
                  Publishing and delivery aren’t connected yet.
                </span>
              </div>
            </>
          )}
        </aside>
      </div>
      {undo && !testing && (
        <div className="wf-undo-bar" role="status">
          <span>{undo.label}</span>
          <button onClick={undoLast}>
            <Icon name="undo" /> Undo
          </button>
        </div>
      )}
      {errors.length > 0 && (
        <div className="wf-readiness">
          <span>
            <Icon name="info" />
            {errors.length} {errors.length === 1 ? "item" : "items"} to
            configure before testing. You can save now.
          </span>
          <button
            className="text-button"
            onClick={() => setShowIssues(!showIssues)}
          >
            {showIssues ? "Hide details" : "Review"}
          </button>
        </div>
      )}
      {showIssues && errors.length > 0 && (
        <div className="wf-validation" role="status">
          <div>
            {errors.map((error, i) => {
              const issue = issues.find((item) => item.message === error);
              return (
                <button
                  key={i}
                  className="wf-issue-link"
                  onClick={() => select(issue?.stepId || "trigger")}
                >
                  {error}
                  <Icon name="arrow" />
                </button>
              );
            })}
          </div>
        </div>
      )}
      <span className="sr-only" role="status">
        {notice}
      </span>
      {adding && (
        <StepPalette
          add={add}
          close={() => setAdding(null)}
          actionsOnly={adding.actionsOnly}
        />
      )}
    </main>
  );
}
