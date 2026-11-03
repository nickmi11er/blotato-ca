import type { Account } from "../model/types";
export function PlatformBadge({
  account,
  small = false,
}: {
  account: Account;
  small?: boolean;
}) {
  return (
    <span
      className={`platform-badge ${account.platform} ${small ? "small-badge" : ""}`}
      aria-label={account.name}
    >
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        {account.platform === "instagram" ? (
          <>
            <rect x="3" y="3" width="18" height="18" rx="5" />
            <circle cx="12" cy="12" r="4" />
            <path d="M17.5 6.5h.01" />
          </>
        ) : account.platform === "facebook" ? (
          <path d="M14 21v-8h3l.5-4H14V7c0-1.1.3-2 2-2h2V2h-3c-3 0-5 1.8-5 5v2H7v4h3v8" />
        ) : (
          <>
            <rect x="2" y="5" width="20" height="14" rx="4" />
            <path d="m10 9 5 3-5 3V9Z" />
          </>
        )}
      </svg>
    </span>
  );
}
