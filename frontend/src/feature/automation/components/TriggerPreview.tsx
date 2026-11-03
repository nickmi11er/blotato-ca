import { useMemo, useState } from "react";
import { Icon } from "../../../shared/ui/Icon";
import { Modal } from "../../../shared/ui/Modal";
import { PlatformBadge } from "./PlatformBadge";
import { WorkflowSteps } from "./WorkflowSteps";
import { ConversationPreview } from "./ConversationPreview";
import { evaluateTrigger } from "../model/matcher";
import type { Draft, ScopeCatalog, TestComment } from "../model/types";
export function TriggerPreview({
  draft,
  catalog,
  initialComment,
}: {
  draft: Draft;
  catalog: ScopeCatalog;
  initialComment: TestComment;
}) {
  const { accounts, posts } = catalog;
  const [sample, setSample] = useState(initialComment);
  const [previewTab, setPreviewTab] = useState<"comment" | "flow">("comment");
  const [conversationOpen, setConversationOpen] = useState(false);
  const [testPulse, setTestPulse] = useState(0);
  const result = useMemo(() => evaluateTrigger(draft, sample), [draft, sample]);
  const active = accounts.filter((account) =>
    draft.accountIds.includes(account.id),
  );
  const dmUnavailable = active.filter((account) => !account.canSendDM);
  const sampleAccount = accounts.find(
    (account) => account.id === sample.accountId,
  );
  const samplePost = posts.find(
    (post) => post.id === sample.postId && post.accountId === sample.accountId,
  );
  if (!sampleAccount || !samplePost)
    return (
      <aside className="preview-column">
        <p>Select an account with a post to preview a comment.</p>
      </aside>
    );
  return (
    <>
      {" "}
      <aside className="preview-column" aria-label="Automation preview">
        <div className="preview-card">
          <div className="preview-heading">
            <span>
              <Icon name="play" /> Live preview
            </span>
            <span className="live-label">
              <i /> PLAYGROUND
            </span>
          </div>
          <div
            className="preview-tabs"
            role="tablist"
            aria-label="Preview mode"
          >
            <button
              id="comment-tab"
              role="tab"
              aria-selected={previewTab === "comment"}
              aria-controls="preview-panel"
              tabIndex={previewTab === "comment" ? 0 : -1}
              onClick={() => setPreviewTab("comment")}
              onKeyDown={(event) => {
                if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
                  event.preventDefault();
                  setPreviewTab("flow");
                  requestAnimationFrame(() =>
                    document.getElementById("flow-tab")?.focus(),
                  );
                }
              }}
            >
              Test your trigger
            </button>
            <button
              id="flow-tab"
              role="tab"
              aria-selected={previewTab === "flow"}
              aria-controls="preview-panel"
              tabIndex={previewTab === "flow" ? 0 : -1}
              onClick={() => setPreviewTab("flow")}
              onKeyDown={(event) => {
                if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
                  event.preventDefault();
                  setPreviewTab("comment");
                  requestAnimationFrame(() =>
                    document.getElementById("comment-tab")?.focus(),
                  );
                }
              }}
            >
              Conversation
            </button>
          </div>
          {previewTab === "comment" ? (
            <div
              id="preview-panel"
              className="tester-body"
              role="tabpanel"
              aria-labelledby="comment-tab"
            >
              <div className="test-source">
                <select
                  aria-label="Test account"
                  value={sample.accountId}
                  onChange={(event) =>
                    setSample({
                      ...sample,
                      accountId: event.target.value,
                      postId:
                        posts.find(
                          (post) => post.accountId === event.target.value,
                        )?.id ?? "",
                    })
                  }
                >
                  {accounts.map((account) => (
                    <option
                      key={account.id}
                      value={account.id}
                      disabled={
                        !posts.some((post) => post.accountId === account.id)
                      }
                    >
                      {account.name}
                    </option>
                  ))}
                </select>
                <select
                  aria-label="Test post"
                  value={sample.postId}
                  onChange={(event) =>
                    setSample({ ...sample, postId: event.target.value })
                  }
                >
                  {posts
                    .filter((post) => post.accountId === sample.accountId)
                    .map((post) => (
                      <option key={post.id} value={post.id}>
                        {post.title}
                      </option>
                    ))}
                </select>
              </div>
              <div className="social-preview">
                <div className="social-profile">
                  <span className="studio-avatar">
                    n<span>✳</span>
                  </span>
                  <div>
                    <strong>northstar.studio</strong>
                    <small>Original content · {samplePost.date}</small>
                  </div>
                  <PlatformBadge account={sampleAccount} small />
                </div>
                <div className={`social-cover ${samplePost.cover}`}>
                  <span className="cover-label">
                    NORTHSTAR / CREATIVE STUDIO
                  </span>
                  <strong>{samplePost.copy}</strong>
                  <span className="cover-orbit" />
                  <span className="cover-note">
                    A little more possibility. <Icon name="arrowUp" />
                  </span>
                </div>
                <div className="social-caption">
                  <strong>northstar.studio</strong> A little something for your
                  next big idea. Comment <span>pricing</span> and we’ll send you
                  the details. ✨
                </div>
                <div className="comment-editor">
                  <span className="comment-avatar">JD</span>
                  <div>
                    <label htmlFor="sample-comment">
                      jordan.design <span>your test comment</span>
                    </label>
                    <textarea
                      id="sample-comment"
                      rows={2}
                      maxLength={2000}
                      spellCheck={false}
                      value={sample.text}
                      onChange={(event) =>
                        setSample({ ...sample, text: event.target.value })
                      }
                    />
                  </div>
                </div>
              </div>
              <div className="quick-samples">
                <span>TRY</span>
                {["pricing", "PRICING ✨", "repricing", "How much?"].map(
                  (text) => (
                    <button
                      key={text}
                      onClick={() => setSample({ ...sample, text })}
                    >
                      {text}
                    </button>
                  ),
                )}
              </div>
              <button
                className="button test-button"
                onClick={() => setTestPulse((value) => value + 1)}
              >
                <Icon name="play" /> Test comment <Icon name="arrow" />
              </button>
              <div
                key={testPulse}
                className={`match-result ${result.matched ? "matched" : "unmatched"}`}
                aria-live="polite"
              >
                <span className="result-icon">
                  <Icon name={result.matched ? "check" : "close"} />
                </span>
                <div>
                  <strong>
                    {result.matched ? "It’s a match!" : "No match this time"}
                  </strong>
                  <p>{result.reason}</p>
                </div>
              </div>
              <details className="test-context">
                <summary>
                  Test context <Icon name="chevron" />
                </summary>
                {(
                  [
                    ["isOwn", "This is my own comment"],
                    ["activeConversation", "Person has an active conversation"],
                    ["previouslyStarted", "Person has started this before"],
                  ] as const
                ).map(([key, text]) => (
                  <label key={key}>
                    <input
                      type="checkbox"
                      checked={sample[key]}
                      onChange={(event) =>
                        setSample({
                          ...sample,
                          [key]: event.target.checked,
                        })
                      }
                    />
                    {text}
                  </label>
                ))}
              </details>
            </div>
          ) : (
            <div
              id="preview-panel"
              className="conversation-tab"
              role="tabpanel"
              aria-labelledby="flow-tab"
            >
              <div className="flow-intro">
                <span className="mini-label">FROM COMMENT TO CONNECTION</span>
                <h3>
                  One comment.
                  <br />
                  <span>Four thoughtful steps.</span>
                </h3>
                <p>
                  Here’s what your customer experiences when the trigger
                  matches.
                </p>
              </div>
              <WorkflowSteps messages={draft.messages} />
              <button
                className="button test-button"
                onClick={() => setConversationOpen(true)}
                disabled={dmUnavailable.length > 0 || active.length === 0}
              >
                <Icon name="play" /> Try the conversation <Icon name="arrow" />
              </button>
              {(dmUnavailable.length > 0 || active.length === 0) && (
                <p className="helper">
                  Select demo accounts with direct messaging to try this flow.
                </p>
              )}
            </div>
          )}
          <div className="preview-footnote">
            <Icon name="info" /> A safe preview. No real messages are sent.
          </div>
        </div>
        <div className="tip-card">
          <Icon name="spark" />
          <div>
            <strong>Small details. Better conversations.</strong>
            <p>
              “Whole word” matches <em>pricing?</em> and skips{" "}
              <em>repricing</em>. Try both in the playground.
            </p>
          </div>
        </div>
      </aside>
      {conversationOpen && (
        <Modal
          title="Meet your next connection."
          eyebrow="CONVERSATION PREVIEW"
          description="Follow the experience from first comment to final link."
          close={() => setConversationOpen(false)}
          className="conversation-modal"
        >
          <ConversationPreview
            messages={draft.messages}
            comment={sample.text}
          />
        </Modal>
      )}
    </>
  );
}
