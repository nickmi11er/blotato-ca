import { defaultWorkflow } from "./workflow";
import type { Draft } from "./types";
export const defaultDraft: Draft = {
  schemaVersion: 2,
  name: "Pricing → email capture",
  accountIds: ["instagram", "facebook"],
  postScope: "all",
  postIds: [],
  matchKind: "keywords",
  operator: "whole_word",
  keywords: ["pricing"],
  logic: "any",
  exclusions: [],
  caseSensitive: false,
  reentry: "skip_active",
  steps: defaultWorkflow(),
};
