// Pure decode half of the Single-Page-Apps-for-GitHub-Pages trick
// (github.com/rafgraph/spa-github-pages) — see public/404.html for the encode half and
// index.js for where this gets applied via history.replaceState. Split out as a pure
// function (pathname/search/hash in, a new path or null out) so the round trip with
// 404.html's own encoding can be unit tested directly, rather than only exercised by
// an actual GitHub Pages deploy — which is exactly how an earlier, broken version of
// this (a "?p=" marker instead of "?/") shipped uncaught: it looked plausible on
// inspection, but "?/" is load-bearing, not an arbitrary choice of query key (see
// below), and nothing but a real round trip through the encoder would have shown that.
//
// The marker is "/?/", not some other custom query string: 404.html's redirect starts
// the query string with a literal "/", so search.slice(1) here already begins with
// "/" too — the whole scheme depends on that "/" being reusable as-is as the start of
// the real path, not on any custom key of ours.
export function decodeGithubPagesRedirectPath(pathname, search, hash) {
  if (search[1] !== "/") return null;
  const decoded = search
    .slice(1)
    .split("&")
    .map((s) => s.replace(/~and~/g, "&"))
    .join("?");
  return pathname.slice(0, -1) + decoded + hash;
}
