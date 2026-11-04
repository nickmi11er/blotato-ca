import { expect, test, vi } from "vitest";
import {
  defaultTemplates,
  readTemplates,
  renderTemplate,
  templateStorageKey,
  validateTemplate,
} from "./templates";

test("templates validate required fields and supported variables", () => {
  const template = defaultTemplates[0];
  expect(validateTemplate(template)).toEqual([]);
  expect(
    validateTemplate({ ...template, subject: "{{unknown}}" }),
  ).toHaveLength(1);
  expect(validateTemplate({ ...template, name: " ", body: " " })).toHaveLength(
    2,
  );
  expect(
    renderTemplate("Hi {{first_name}}, {{email}}", "a@example.com", "Alex"),
  ).toBe("Hi Alex, a@example.com");
});

test("empty collections persist while invalid and unavailable storage recovers", () => {
  localStorage.setItem(templateStorageKey, "[]");
  expect(readTemplates()).toEqual([]);
  localStorage.setItem(
    templateStorageKey,
    JSON.stringify([defaultTemplates[0], defaultTemplates[0]]),
  );
  expect(readTemplates()).toEqual(defaultTemplates);
  localStorage.setItem(templateStorageKey, "bad json");
  expect(readTemplates()).toEqual(defaultTemplates);
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
    throw new Error("blocked");
  });
  expect(readTemplates()).toEqual(defaultTemplates);
});
