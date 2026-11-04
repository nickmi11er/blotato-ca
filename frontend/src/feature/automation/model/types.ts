import type { Step } from "./workflow";
export type MatchOperator = "whole_word" | "exact" | "contains";
export type MatchKind = "keywords" | "any";
export type Reentry = "skip_active" | "once" | "every";
export interface Draft {
  schemaVersion: 2;
  name: string;
  priority?: number;
  accountIds: string[];
  postScope: "all" | "selected";
  scopeModes?: Record<string, "all" | "selected">;
  postIds: string[];
  matchKind: MatchKind;
  operator: MatchOperator;
  keywords: string[];
  logic: "any" | "all";
  exclusions: string[];
  caseSensitive: boolean;
  reentry: Reentry;
  steps: Step[];
}
export interface TestComment {
  accountId: string;
  postId: string;
  text: string;
  isOwn: boolean;
  activeConversation: boolean;
  previouslyStarted: boolean;
}
export interface MatchResult {
  matched: boolean;
  reason: string;
  terms?: string[];
}
export type Platform = "instagram" | "facebook" | "youtube";
export interface Account {
  id: string;
  name: string;
  handle: string;
  platform: Platform;
  canSendDM: boolean;
}
export interface Post {
  id: string;
  accountId: string;
  title: string;
  date: string;
  cover: string;
  copy: string;
  comments: number;
}

export interface ScopeCatalog {
  accounts: readonly Account[];
  posts: readonly Post[];
}
export type UpdateDraft = (patch: Partial<Draft>) => void;
