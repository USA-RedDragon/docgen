// Decodes zstd data with unzstd.wasm (Go, internal/zstd), from the URL in the wasm parameter of the
// script URL. Message in: {id, data: Uint8Array, size}. Out: {id, out} or {id, error}.
"use strict";
importScripts("wasm_exec.js");
const queue = [];
let ready = false;
const run = ({ id, data, size }) => {
  const t0 = performance.now();
  const r = self.zstdDecode(data, size);
  if (r instanceof Error) postMessage({ id, error: r.message });
  else postMessage({ id, out: r, ms: performance.now() - t0 }, [r.buffer]);
};
self.zstdReady = () => { ready = true; queue.splice(0).forEach(run); };
self.onmessage = e => { if (ready) run(e.data); else queue.push(e.data); };
const go = new Go();
const wasm = new URL(location.href).searchParams.get("wasm");
WebAssembly.instantiateStreaming(fetch(wasm), go.importObject)
  .then(r => go.run(r.instance))
  .catch(e => postMessage({ id: 0, error: wasm + ": " + e }));
