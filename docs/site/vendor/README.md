# Vendored CodeMirror

`codemirror.js` is one minified ESM bundle of CodeMirror 6. `docs/site/build.sh` copies it to the root of a site that has a page with an editor block. It comes from `codemirror.entry.js`, and `LICENSE` is the MIT license that the bundle holds as its first comment.

The bundle uses `markdownLanguage` and not `markdown()`, so that it holds no HTML, CSS or JavaScript parser. That saves 60 KB after gzip.

Direct packages:

| Package | Version |
| --- | --- |
| `@codemirror/state` | 6.7.6 |
| `@codemirror/view` | 6.43.15 |
| `@codemirror/language` | 6.13.1 |
| `@codemirror/lang-go` | 6.0.1 |
| `@codemirror/lang-yaml` | 6.1.3 |
| `@codemirror/lang-markdown` | 6.5.2 |
| `@codemirror/lang-cpp` | 6.0.3 |
| `@lezer/highlight` | 1.2.5 |
| `esbuild` | 0.28.2 |

The other packages in the bundle:

| Package | Version |
| --- | --- |
| `@codemirror/autocomplete` | 6.20.4 |
| `@lezer/common` | 1.5.5 |
| `@lezer/cpp` | 1.1.6 |
| `@lezer/go` | 1.0.1 |
| `@lezer/lr` | 1.4.12 |
| `@lezer/markdown` | 1.8.1 |
| `@lezer/yaml` | 1.0.4 |
| `@marijn/find-cluster-break` | 1.0.4 |
| `style-mod` | 4.1.4 |
| `w3c-keyname` | 2.2.8 |

To build the bundle, run these lines in an empty directory outside the repository, with `$repo` set to the repository root:

```sh
npm init -y
npm install --ignore-scripts --save-exact @codemirror/state@6.7.6 @codemirror/view@6.43.15 @codemirror/language@6.13.1 @codemirror/lang-go@6.0.1 @codemirror/lang-yaml@6.1.3 @codemirror/lang-markdown@6.5.2 @codemirror/lang-cpp@6.0.3 @lezer/highlight@1.2.5 esbuild@0.28.2
cp "$repo/docs/site/vendor/codemirror.entry.js" .
npx esbuild codemirror.entry.js --bundle --format=esm --minify --banner:js="/*$(cat "$repo/docs/site/vendor/LICENSE")*/" --outfile="$repo/docs/site/vendor/codemirror.js"
```

The SHA-256 of the result for these versions is `7d9f73a6f68deaa8f32d64ffabc9dbcc392f75347ddd853b1e69b0adeefaf889`.
