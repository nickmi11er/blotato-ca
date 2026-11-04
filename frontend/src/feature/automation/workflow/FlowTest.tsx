import { useState, useEffect, useRef } from "react";
import { Icon } from "../../../shared/ui/Icon";
import type { EmailTemplate } from "../../email-templates/model/templates";
import type { Draft, ScopeCatalog } from "../model/types";
import { evaluateTrigger } from "../model/matcher";
import {
  resumeSimulation,
  startSimulation,
  type Simulation,
} from "../model/simulation";
import { validateDraft } from "../model/validation";
export function FlowTest({
  draft,
  templates,
  catalog,
  close,
  run,
  setRun,
}: {
  draft: Draft;
  templates: readonly EmailTemplate[];
  catalog: ScopeCatalog;
  close: () => void;
  run: Simulation | null;
  setRun: (run: Simulation | null) => void;
}) {
  const eventsRef = useRef<HTMLOListElement>(null);
  useEffect(() => {
    const list = eventsRef.current;
    if (list) list.scrollTop = list.scrollHeight;
  }, [run?.events.length]);
  const [comment, setComment] = useState("Could you send me your pricing?");
  const [accountId, setAccountId] = useState(
    draft.accountIds[0] ?? catalog.accounts[0]?.id ?? "",
  );
  const availablePosts = catalog.posts.filter((p) => p.accountId === accountId);
  const [postId, setPostId] = useState(availablePosts[0]?.id ?? "");
  const [reply, setReply] = useState("");
  const [result, setResult] = useState("");
  const errors = validateDraft(draft, catalog, templates);
  const active = run ? run.sequence[run.cursor] : undefined;
  function start() {
    const match = evaluateTrigger(draft, {
      accountId,
      postId,
      text: comment,
      isOwn: false,
      activeConversation: false,
      previouslyStarted: false,
    });
    setResult(match.reason);
    setReply("");
    setRun(match.matched ? startSimulation(draft.steps, templates) : null);
  }
  return (
    <section className="wf-test-panel" aria-label="Flow test">
      <div className="wf-test-title">
        <span className="mini-label">TEST RUN</span>
        <button className="text-button" onClick={close}>
          <Icon name="close" /> Exit test
        </button>
      </div>
      <h2>See your flow in action.</h2>
      <p className="wf-help">
        A local simulation. No messages are sent. Watch the highlighted steps as
        you reply or advance time.
      </p>
      {!run && (
        <div className="wf-fields">
          <div className="wf-field-row">
            <label>
              Social account
              <select
                value={accountId}
                onChange={(e) => {
                  setAccountId(e.target.value);
                  setPostId(
                    catalog.posts.find((p) => p.accountId === e.target.value)
                      ?.id ?? "",
                  );
                }}
              >
                {catalog.accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Post
              <select
                value={postId}
                onChange={(e) => setPostId(e.target.value)}
              >
                {availablePosts.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.title}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <label>
            Incoming comment
            <textarea
              rows={2}
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
          </label>
          <button
            className="button button-primary"
            onClick={start}
            disabled={errors.length > 0}
          >
            <Icon name="play" /> Start test
          </button>
          {errors.length > 0 && (
            <p className="wf-error">
              Fix your flow before testing: {errors[0]}
            </p>
          )}
          {result && (
            <p role="status" className="wf-tip">
              {result}
            </p>
          )}
        </div>
      )}
      {run && (
        <>
          <div className="wf-run-head">
            <span className={`wf-run-state ${run.status}`}>
              <i />
              {run.status === "waiting"
                ? "Session waiting"
                : run.status === "complete"
                  ? "Completed"
                  : "Session ended"}
            </span>
            <button
              className="text-button"
              onClick={() => {
                setRun(null);
                setResult("");
              }}
            >
              Restart test
            </button>
          </div>
          <div className="wf-test-contact">
            <strong>Test contact · Jordan</strong>
            <span>Email: {run.email || "Not captured yet"}</span>
            {run.status === "waiting" && run.wakeAt && (
              <span>Resumes: {new Date(run.wakeAt).toLocaleString()}</span>
            )}
          </div>
          <ol ref={eventsRef} className="wf-events" aria-live="polite">
            {run.events.map((event, index) => (
              <li key={index} className={event.tone}>
                <span className="wf-event-dot" />
                <div>
                  <strong>{event.label}</strong>
                  <p>{event.text}</p>
                </div>
              </li>
            ))}
          </ol>
          {run.status === "waiting" && active?.kind === "wait_reply" && (
            <form
              className="wf-test-reply"
              onSubmit={(e) => {
                e.preventDefault();
                setRun(
                  resumeSimulation(run, templates, {
                    kind: "reply",
                    text: reply,
                  }),
                );
                setReply("");
              }}
            >
              <label>
                Simulate an incoming DM
                <input
                  value={reply}
                  placeholder="e.g. My email is jordan@example.com"
                  onChange={(e) => setReply(e.target.value)}
                />
              </label>
              <div className="wf-test-actions">
                <button
                  type="submit"
                  className="button button-primary"
                  disabled={!reply.trim()}
                >
                  <Icon name="send" /> Send reply
                </button>
                <button
                  type="button"
                  className="text-button"
                  onClick={() =>
                    setRun(
                      resumeSimulation(run, templates, {
                        kind: "timeout",
                      }),
                    )
                  }
                >
                  Simulate timeout
                </button>
              </div>
            </form>
          )}
          {run.status === "waiting" &&
            (active?.kind === "delay" || active?.kind === "schedule") && (
              <button
                className="button button-primary"
                onClick={() =>
                  setRun(
                    resumeSimulation(run, templates, {
                      kind: "timer",
                    }),
                  )
                }
              >
                <Icon name="clock" /> Advance clock & continue
              </button>
            )}
        </>
      )}
    </section>
  );
}
