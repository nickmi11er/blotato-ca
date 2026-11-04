import { useState } from "react";
import {
  readTemplates,
  templateStorageKey,
  validateTemplate,
  type EmailTemplate,
} from "./templates";
export function useTemplates() {
  const [templates, setTemplates] = useState(readTemplates);
  function persist(next: EmailTemplate[]): string | null {
    try {
      localStorage.setItem(templateStorageKey, JSON.stringify(next));
      setTemplates(next);
      return null;
    } catch {
      return "Browser storage is unavailable. Your changes have not been saved.";
    }
  }
  function save(template: EmailTemplate) {
    const errors = validateTemplate(template);
    if (errors.length) return errors[0];
    if (templates.length >= 100 && !templates.some((t) => t.id === template.id))
      return "You can keep up to 100 templates on this device.";
    return persist(
      templates.some((t) => t.id === template.id)
        ? templates.map((t) => (t.id === template.id ? template : t))
        : [...templates, template],
    );
  }
  return {
    templates,
    save,
    remove: (id: string) => persist(templates.filter((t) => t.id !== id)),
  };
}
