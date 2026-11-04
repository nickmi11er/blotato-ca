import { useSyncExternalStore } from "react";

function subscribe(onChange: () => void) {
  window.addEventListener("popstate", onChange);
  return () => window.removeEventListener("popstate", onChange);
}
function selectedId() {
  return new URLSearchParams(window.location.search).get("automation");
}

// Keep identity in the URL independently of the current workspace section.
// Reload, copied links and returning from templates retain the same editor.
export function useAutomationSelection(fallback: string) {
  const id = useSyncExternalStore(subscribe, selectedId, () => null);
  function select(next: string) {
    const url = new URL(window.location.href);
    if (next) url.searchParams.set("automation", next);
    else url.searchParams.delete("automation");
    window.history.replaceState(null, "", url);
    window.dispatchEvent(new PopStateEvent("popstate"));
  }
  return [id ?? fallback, select] as const;
}
