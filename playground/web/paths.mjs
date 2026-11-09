// Path helpers for the slash-separated, absolute paths of the memfs.

/** normalizePath returns the clean absolute form of path, relative to the directory cwd. */
export function normalizePath(path, cwd = "/") {
  if (!path.startsWith("/")) path = cwd + "/" + path;
  const parts = [];
  for (const part of path.split("/")) {
    if (!part || part === ".") continue;
    if (part === "..") parts.pop();
    else parts.push(part);
  }
  return "/" + parts.join("/");
}

/** parentOf returns the directory of path: "/a/b" for "/a/b/c", and "" for "/a". */
export const parentOf = path => path.slice(0, path.lastIndexOf("/"));

/** baseName returns the last element of path. */
export const baseName = path => path.slice(path.lastIndexOf("/") + 1);

/** isUnder reports whether path is dir or a path inside dir. */
export const isUnder = (path, dir) => path === dir || path.startsWith(dir + "/");

/** movePath returns path after the moves of pairs, each [from, to], or path when no move covers it. */
export function movePath(path, pairs) {
  for (const [from, to] of pairs) {
    if (isUnder(path, from)) return to + path.slice(from.length);
  }
  return path;
}
