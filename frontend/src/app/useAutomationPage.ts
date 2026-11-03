import { useSyncExternalStore } from "react";

function subscribe(onChange: () => void) {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}

function getPage() {
  return window.location.hash === "#automations" ? "automations" : "builder";
}

export function useAutomationPage() {
  return useSyncExternalStore(subscribe, getPage, () => "builder");
}
