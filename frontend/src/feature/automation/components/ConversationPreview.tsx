import { useState } from "react";
import { Icon } from "../../../shared/ui/Icon";
import type { Messages } from "../model/types";
export function ConversationPreview({
  messages,
  comment,
}: {
  messages: Messages;
  comment: string;
}) {
  const [email, setEmail] = useState("");
  const [captured, setCaptured] = useState(false);
  return (
    <>
      <div className="public-reply-preview">
        <span className="comment-avatar">JD</span>
        <div>
          <strong>jordan.design</strong>
          <p>{comment}</p>
          <div className="public-reply">
            <span className="studio-avatar small">n✳</span>
            <div>
              <strong>northstar.studio</strong>
              <p>{messages.reply}</p>
            </div>
          </div>
        </div>
      </div>
      <div className="dm-preview">
        <div className="dm-header">
          <Icon name="mail" /> YOUR PRIVATE CONVERSATION <span>Demo</span>
        </div>
        <div className="chat-bubble incoming">{messages.question}</div>
        {captured ? (
          <>
            <div className="chat-bubble outgoing">{email}</div>
            <div className="email-captured">
              <Icon name="check" /> Email captured
            </div>
            <div className="chat-bubble incoming">
              {messages.linkText}
              <a href={messages.link} target="_blank" rel="noopener noreferrer">
                <Icon name="link" />
                {messages.link}
                <Icon name="external" />
              </a>
            </div>
            <div className="conversation-complete">
              <Icon name="check" /> Conversation complete. A new connection
              made.
            </div>
          </>
        ) : (
          <>
            <div className="waiting-label">
              <span /> Waiting for an email address
            </div>
            <form
              className="email-form"
              onSubmit={(event) => {
                event.preventDefault();
                setCaptured(true);
              }}
            >
              <label className="sr-only" htmlFor="preview-email">
                Your test email address
              </label>
              <input
                id="preview-email"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="Try an email address…"
                required
                autoComplete="off"
                maxLength={254}
              />
              <button
                className="button button-primary"
                aria-label="Send test email"
                type="submit"
              >
                <Icon name="send" />
              </button>
            </form>
          </>
        )}
      </div>
      <p className="dialog-footnote">
        <Icon name="info" /> Demo conversation. No emails or messages are sent.
      </p>
    </>
  );
}
