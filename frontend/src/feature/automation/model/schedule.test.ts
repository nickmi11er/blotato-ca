import { expect, test } from "vitest";
import { nextTimeInstant } from "./schedule";
const at = (value: string) => Date.parse(value);
test("next time uses its saved timezone, rolling over after the chosen minute", () => {
  expect(
    nextTimeInstant("09:00", "America/New_York", at("2030-01-01T13:00:00Z")),
  ).toBe(at("2030-01-01T14:00:00Z"));
  expect(
    nextTimeInstant("09:00", "America/New_York", at("2030-01-01T14:00:01Z")),
  ).toBe(at("2030-01-02T14:00:00Z"));
  expect(nextTimeInstant("25:30", "UTC", Date.now())).toBeNull();
  expect(nextTimeInstant("09:00", "invalid/zone", Date.now())).toBeNull();
});
test("daylight saving skips missing times and uses the next real occurrence of repeated times", () => {
  expect(
    nextTimeInstant("02:30", "America/New_York", at("2026-03-08T06:00:00Z")),
  ).toBe(at("2026-03-09T06:30:00Z"));
  expect(
    nextTimeInstant("01:30", "America/New_York", at("2026-11-01T05:00:00Z")),
  ).toBe(at("2026-11-01T05:30:00Z"));
  expect(
    nextTimeInstant("01:30", "America/New_York", at("2026-11-01T05:45:00Z")),
  ).toBe(at("2026-11-01T06:30:00Z"));
});
