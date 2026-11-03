import { Icon } from "../../../shared/ui/Icon";
import { Modal } from "../../../shared/ui/Modal";
import { PlatformBadge } from "./PlatformBadge";
import { MessageEditor } from "./MessageEditor";
import type { Draft, ScopeCatalog, UpdateDraft } from "../model/types";
export type EditorModal =
  "rename" | "messages" | "accounts" | "activity" | null;
export function AutomationDialogs({
  modal,
  setModal,
  draft,
  update,
  savedAt,
  setToast,
  catalog,
}: {
  modal: EditorModal;
  setModal: (modal: EditorModal) => void;
  draft: Draft;
  update: UpdateDraft;
  savedAt: string | null;
  setToast: (message: string) => void;
  catalog: ScopeCatalog;
}) {
  const { accounts } = catalog;
  return (
    <>
      {" "}
      {modal === "rename" && (
        <Modal
          title="Name your automation"
          eyebrow="MAKE IT YOURS"
          description="A little clarity goes a long way."
          close={() => setModal(null)}
        >
          <form
            className="modal-form"
            onSubmit={(event) => {
              event.preventDefault();
              const name = String(
                new FormData(event.currentTarget).get("name"),
              ).trim();
              if (name) {
                update({ name });
                setModal(null);
              }
            }}
          >
            <label>
              Automation name
              <input
                name="name"
                defaultValue={draft.name}
                maxLength={100}
                required
                autoFocus
              />
            </label>
            <button className="button button-primary" type="submit">
              Save name <Icon name="check" />
            </button>
          </form>
        </Modal>
      )}
      {modal === "messages" && (
        <Modal
          title="Make it sound like you."
          eyebrow="THE FOLLOW-UP"
          description="These messages are used in the conversation preview."
          close={() => setModal(null)}
        >
          <MessageEditor
            messages={draft.messages}
            onSave={(messages) => {
              update({ messages });
              setModal(null);
              setToast(
                "Messages updated. Try them in the conversation preview.",
              );
            }}
          />
        </Modal>
      )}
      {modal === "accounts" && (
        <Modal
          title="One workspace. More conversations."
          eyebrow="YOUR CHANNELS"
          description="Demo accounts to explore your trigger across channels."
          close={() => setModal(null)}
        >
          <div className="dialog-accounts">
            {accounts.map((account) => (
              <div key={account.id}>
                <PlatformBadge account={account} />
                <span>
                  <strong>{account.name}</strong>
                  <small>{account.handle}</small>
                </span>
                <span className="account-state">
                  {draft.accountIds.includes(account.id)
                    ? "Selected"
                    : "Available"}
                </span>
              </div>
            ))}
          </div>
          <button
            className="button button-primary"
            onClick={() => setModal(null)}
          >
            Back to your trigger <Icon name="arrow" />
          </button>
        </Modal>
      )}
      {modal === "activity" && (
        <Modal
          title="Your story starts here."
          eyebrow="WORKSPACE ACTIVITY"
          description="This is a design preview. Real activity will appear when automations are connected to the backend."
          close={() => setModal(null)}
        >
          <div className="activity-entry">
            <Icon name={savedAt ? "save" : "edit"} />
            <div>
              <strong>
                {savedAt
                  ? "Draft saved on this device"
                  : "Your first automation is taking shape"}
              </strong>
              <small>{draft.name}</small>
            </div>
            <span className="draft-badge">Draft</span>
          </div>
          <button
            className="button button-primary"
            onClick={() => setModal(null)}
          >
            Keep creating <Icon name="arrow" />
          </button>
        </Modal>
      )}
    </>
  );
}
