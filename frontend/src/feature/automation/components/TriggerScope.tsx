import { Icon } from "../../../shared/ui/Icon";
import { Section } from "../../../shared/ui/Section";
import { PlatformBadge } from "./PlatformBadge";
import type { Draft, ScopeCatalog, UpdateDraft } from "../model/types";
export function TriggerScope({
  draft,
  update,
  catalog,
  toggleAccount,
}: {
  draft: Draft;
  update: UpdateDraft;
  catalog: ScopeCatalog;
  toggleAccount: (id: string) => void;
}) {
  const { accounts, posts } = catalog;
  const visiblePosts = posts.filter((post) =>
    draft.accountIds.includes(post.accountId),
  );
  const dmUnavailable = accounts.filter(
    (account) => draft.accountIds.includes(account.id) && !account.canSendDM,
  );
  const unavailableAccounts = draft.accountIds.filter(
    (id) => !accounts.some((account) => account.id === id),
  );
  const unavailablePosts = draft.postIds.filter(
    (id) =>
      !posts.some(
        (post) => post.id === id && draft.accountIds.includes(post.accountId),
      ),
  );
  return (
    <Section
      number="01"
      title="Choose your starting point"
      subtitle="A new comment on any of your connected channels."
      icon="comment"
    >
      <div className="field-heading">
        <span id="accounts-label">Social accounts</span>
        <span>Select one or more</span>
      </div>
      <div
        className="account-grid"
        role="group"
        aria-labelledby="accounts-label"
      >
        {accounts.map((account) => (
          <label
            className={`account-card ${draft.accountIds.includes(account.id) ? "selected" : ""}`}
            key={account.id}
          >
            <input
              className="sr-only"
              type="checkbox"
              aria-label={`${account.name} ${account.handle}`}
              checked={draft.accountIds.includes(account.id)}
              onChange={() => toggleAccount(account.id)}
            />
            <PlatformBadge account={account} />
            <span className="account-copy">
              <strong>{account.name}</strong>
              <small>{account.handle}</small>
            </span>
            <span className="selection-check">
              <Icon name="check" />
            </span>
          </label>
        ))}
      </div>
      {unavailableAccounts.map((id) => (
        <p className="validation-note" key={id}>
          Unavailable account: {id}
          <button
            className="text-button"
            onClick={() => toggleAccount(id)}
            aria-label={`Remove unavailable account ${id}`}
          >
            Remove
          </button>
        </p>
      ))}
      <div className="field-heading post-field">
        <span>Which posts?</span>
        <span>
          {draft.postScope === "all"
            ? "Including future posts"
            : `${draft.postIds.length} selected`}
        </span>
      </div>
      <div className="segmented" role="group" aria-label="Post scope">
        <button
          onClick={() => update({ postScope: "all" })}
          aria-pressed={draft.postScope === "all"}
        >
          <Icon name="grid" /> All posts
        </button>
        <button
          onClick={() => update({ postScope: "selected" })}
          aria-pressed={draft.postScope === "selected"}
        >
          <Icon name="check" /> Specific posts
        </button>
      </div>
      {draft.postScope === "selected" &&
        unavailablePosts.map((id) => (
          <p className="validation-note" key={id}>
            Unavailable post: {id}
            <button
              className="text-button"
              onClick={() =>
                update({
                  postIds: draft.postIds.filter((postId) => postId !== id),
                })
              }
              aria-label={`Remove unavailable post ${id}`}
            >
              Remove
            </button>
          </p>
        ))}
      {draft.scopeModes && (
        <div className="wf-fields">
          <p className="helper">
            This flow uses a different post scope for each account.
          </p>
          {draft.accountIds.map((id) => (
            <label key={id}>
              Posts for {catalog.accounts.find((a) => a.id === id)?.name ?? id}
              <select
                value={draft.scopeModes?.[id] ?? draft.postScope}
                onChange={(e) =>
                  update({
                    scopeModes: {
                      ...draft.scopeModes,
                      [id]: e.target.value as "all" | "selected",
                    },
                  })
                }
              >
                <option value="all">All posts</option>
                <option value="selected">Selected posts</option>
              </select>
            </label>
          ))}
        </div>
      )}
      {draft.postScope === "selected" ? (
        <div className="post-grid">
          {visiblePosts.length ? (
            visiblePosts
              .filter(
                (post) =>
                  !draft.scopeModes ||
                  (draft.scopeModes[post.accountId] ?? draft.postScope) ===
                    "selected",
              )
              .map((post) => (
                <label
                  className={`post-card ${draft.postIds.includes(post.id) ? "selected" : ""}`}
                  key={post.id}
                >
                  <input
                    className="sr-only"
                    type="checkbox"
                    checked={draft.postIds.includes(post.id)}
                    onChange={() =>
                      update({
                        postIds: draft.postIds.includes(post.id)
                          ? draft.postIds.filter((id) => id !== post.id)
                          : [...draft.postIds, post.id],
                      })
                    }
                    aria-label={`Select post: ${post.title}`}
                  />
                  <span className={`post-cover ${post.cover}`}>
                    <span>{post.copy}</span>
                    <span className="post-check">
                      <Icon name="check" />
                    </span>
                  </span>
                  <span className="post-caption">{post.title}</span>
                  <span className="post-date">{post.date}</span>
                </label>
              ))
          ) : (
            <p className="helper">
              No posts available for the selected accounts.
            </p>
          )}
        </div>
      ) : (
        <div className="scope-note">
          <Icon name="info" /> Comments on existing and future posts can start
          this automation.
        </div>
      )}
      {dmUnavailable.length > 0 && (
        <div className="capability-note" role="status">
          <Icon name="info" />
          <p>
            DM steps are unavailable for:{" "}
            {dmUnavailable.map((account) => account.name).join(", ")}. You can
            configure its comment trigger; email capture needs a compatible
            account.
          </p>
        </div>
      )}
    </Section>
  );
}
