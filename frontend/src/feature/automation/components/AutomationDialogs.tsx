import { Icon } from "../../../shared/ui/Icon";
import { Modal } from "../../../shared/ui/Modal";
import type { Draft, UpdateDraft } from "../model/types";
export type EditorModal = "rename" | null;
export function AutomationDialogs({
  modal,
  setModal,
  draft,
  update,
}: {
  modal: EditorModal;
  setModal: (modal: EditorModal) => void;
  draft: Draft;
  update: UpdateDraft;
}) {
  return (
    <>
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
    </>
  );
}
