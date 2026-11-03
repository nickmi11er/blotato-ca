import type { Draft } from "./types";
export const defaultDraft: Draft = {
  schemaVersion: 1,
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
  messages: {
    reply: "Sent you a DM! ✨",
    question:
      "Hey! Thanks for your interest. What’s the best email address to send our pricing to?",
    linkText: "You’re all set! Here’s a little more about our pricing:",
    link: "https://example.com/pricing",
  },
};
