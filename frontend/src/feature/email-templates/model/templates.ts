export interface EmailTemplate {
  id: string;
  name: string;
  subject: string;
  body: string;
}
export const defaultTemplates: EmailTemplate[] = [
  {
    id: "pricing-intro",
    name: "Pricing & next steps",
    subject: "A little more possibility, {{first_name}}",
    body: "Hi {{first_name}},\n\nThanks for reaching out! Here’s everything you need to find the right plan for you.\n\nExplore our pricing: https://example.com/pricing\n\nHave a question? Just hit reply. We’d love to help.\n\nThe Northstar team",
  },
  {
    id: "welcome-note",
    name: "A warm welcome",
    subject: "You’re in, {{first_name}} ✨",
    body: "Hi {{first_name}},\n\nSo glad you’re here. This is the start of something good.\n\nWe’ll be in touch with ideas, updates, and a little inspiration.\n\nThe Northstar team",
  },
];
export function validateTemplate(template: EmailTemplate): string[] {
  const errors: string[] = [];
  if (!template.id || template.id.length > 100)
    errors.push("This template has an invalid identifier.");
  if (!template.name.trim()) errors.push("Give this template a name.");
  if (!template.subject.trim()) errors.push("Add an email subject.");
  if (!template.body.trim()) errors.push("Write the email content.");
  if (
    template.name.length > 100 ||
    template.subject.length > 200 ||
    template.body.length > 10000
  )
    errors.push("This template exceeds the supported length.");
  const variables =
    `${template.subject} ${template.body}`.match(/\{\{[^}]+\}\}/g) ?? [];
  if (variables.some((v) => !["{{first_name}}", "{{email}}"].includes(v)))
    errors.push("Use the supported variables {{first_name}} and {{email}}.");
  return errors;
}
export function renderTemplate(
  text: string,
  email = "jordan@example.com",
  firstName = "Jordan",
) {
  return text
    .replaceAll("{{first_name}}", firstName)
    .replaceAll("{{email}}", email);
}
export const templateStorageKey = "blotato.email-templates.v1";
export function readTemplates(): EmailTemplate[] {
  try {
    const value: unknown = JSON.parse(
      localStorage.getItem(templateStorageKey) || "null",
    );
    if (
      Array.isArray(value) &&
      value.length <= 100 &&
      value.every(
        (t) =>
          t &&
          typeof t === "object" &&
          typeof t.id === "string" &&
          typeof t.name === "string" &&
          typeof t.subject === "string" &&
          typeof t.body === "string" &&
          validateTemplate(t).length === 0,
      ) &&
      new Set(value.map((t) => t.id)).size === value.length
    )
      return value;
  } catch {
    /* The editor works even if local storage is unavailable. */
  }
  return structuredClone(defaultTemplates);
}
