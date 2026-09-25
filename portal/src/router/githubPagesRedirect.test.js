import { describe, it, expect } from "vitest";
import { decodeGithubPagesRedirectPath } from "./githubPagesRedirect";

// Mirrors public/404.html's redirect exactly (same segmentsToKeep = 1) so tests here
// exercise the real round trip, not just the decoder in isolation — the shipped bug
// (a "?p=" marker instead of "?/") passed a decoder-only sanity check just fine; only
// running the full encode -> decode round trip catches that "/?/" is load-bearing.
function encode(pathname, search, hash) {
  const segmentsToKeep = 1;
  const kept = pathname.split("/").slice(0, 1 + segmentsToKeep).join("/");
  const remainder = pathname.slice(1).split("/").slice(segmentsToKeep).join("/").replace(/&/g, "~and~");
  const q = search ? "&" + search.slice(1).replace(/&/g, "~and~") : "";
  return { pathname: kept + "/", search: "?/" + remainder + q, hash };
}

describe("decodeGithubPagesRedirectPath", () => {
  it("round-trips a plain path through 404.html's own encoding", () => {
    const redirected = encode("/fleet-planner/buses", "", "");
    const result = decodeGithubPagesRedirectPath(redirected.pathname, redirected.search, redirected.hash);
    expect(result).toBe("/fleet-planner/buses");
  });

  it("round-trips a deeper path", () => {
    const redirected = encode("/fleet-planner/trips/42/edit", "", "");
    const result = decodeGithubPagesRedirectPath(redirected.pathname, redirected.search, redirected.hash);
    expect(result).toBe("/fleet-planner/trips/42/edit");
  });

  it("preserves an original query string", () => {
    const redirected = encode("/fleet-planner/timeline", "?view=week&date=2026-09-25", "");
    const result = decodeGithubPagesRedirectPath(redirected.pathname, redirected.search, redirected.hash);
    expect(result).toBe("/fleet-planner/timeline?view=week&date=2026-09-25");
  });

  it("preserves a hash", () => {
    const redirected = encode("/fleet-planner/buses", "", "#section");
    const result = decodeGithubPagesRedirectPath(redirected.pathname, redirected.search, redirected.hash);
    expect(result).toBe("/fleet-planner/buses#section");
  });

  it("is idempotent — decoding an already-decoded (real) URL is a no-op", () => {
    // Regression guard for the actual shipped bug: a "?p=" marker made every refresh
    // re-mangle the path into "p=p=p=p=...", compounding forever instead of landing
    // once. A correct decoder must leave a normal, already-resolved URL alone.
    const result = decodeGithubPagesRedirectPath("/fleet-planner/buses", "", "");
    expect(result).toBeNull();
  });

  it("does not fire on an unrelated query string", () => {
    const result = decodeGithubPagesRedirectPath("/fleet-planner/", "?foo=bar", "");
    expect(result).toBeNull();
  });
});
