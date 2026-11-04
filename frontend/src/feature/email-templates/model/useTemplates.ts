import { useRef, useState } from "react";
import {
  errorMessage,
  request,
  versionHeader,
} from "../../../shared/api/client";
import { validateTemplate, type EmailTemplate } from "./templates";
export function useTemplates(initial: EmailTemplate[]) {
  const [templates, setTemplates] = useState(initial);
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  async function mutate(action: () => Promise<void>): Promise<string | null> {
    if (lock.current) return "Another request is still in progress.";
    lock.current = true;
    setBusy(true);
    try {
      await action();
      return null;
    } catch (error) {
      return errorMessage(error);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  async function save(template: EmailTemplate) {
    const errors = validateTemplate(template);
    if (errors.length) return errors[0];
    return mutate(async () => {
      const revision = template.revision ?? 0;
      await request(`templates/${encodeURIComponent(template.id)}`, {
        method: "PUT",
        body: JSON.stringify({ ...template, revision }),
      });
      const saved = { ...template, revision: revision + 1 };
      setTemplates((previous) =>
        previous.some((t) => t.id === saved.id)
          ? previous.map((t) => (t.id === saved.id ? saved : t))
          : [...previous, saved],
      );
    });
  }
  function remove(id: string) {
    return mutate(async () => {
      const template = templates.find((t) => t.id === id);
      await request(`templates/${encodeURIComponent(id)}`, {
        method: "DELETE",
        headers: versionHeader(template?.revision ?? 0),
      });
      setTemplates((previous) => previous.filter((t) => t.id !== id));
    });
  }
  return { templates, busy, save, remove };
}
