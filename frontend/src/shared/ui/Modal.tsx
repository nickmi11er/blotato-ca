import { useEffect, useRef, useId, type ReactNode } from "react";
import { Icon } from "./Icon";
export function Modal({
  title,
  eyebrow,
  description,
  children,
  close,
  className = "",
}: {
  title: string;
  eyebrow: string;
  description: string;
  children: ReactNode;
  close: () => void;
  className?: string;
}) {
  const titleId = useId();
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className={`modal ${className}`}
      aria-labelledby={titleId}
      onClose={close}
      onClick={(event) => {
        if (event.target !== ref.current) return;
        const rect = ref.current.getBoundingClientRect();
        if (
          event.clientX < rect.left ||
          event.clientX > rect.right ||
          event.clientY < rect.top ||
          event.clientY > rect.bottom
        )
          ref.current.close();
      }}
    >
      <button
        className="modal-close"
        onClick={() => ref.current?.close()}
        aria-label="Close dialog"
      >
        <Icon name="close" />
      </button>
      <span className="dialog-eyebrow">{eyebrow}</span>
      <h2 id={titleId}>{title}</h2>
      <p className="dialog-description">{description}</p>
      {children}
    </dialog>
  );
}
