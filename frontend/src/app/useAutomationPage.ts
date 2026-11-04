import { useSyncExternalStore } from "react";

function subscribe(onChange: () => void) {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}

function getPage() {
  return window.location.hash === "#templates"
    ? "templates"
    : window.location.hash === "#automations"
      ? "automations"
      : window.location.hash === "#builder"
        ? "builder"
        : "automations";
}

export function useAutomationPage() {
  return useSyncExternalStore(subscribe, getPage, () => "builder");
}
