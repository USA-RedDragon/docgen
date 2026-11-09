// Marionette client for headless Firefox. Usage from a scenario: const ff = await connect(port); await ff.go(url); ...
import net from "node:net";
import fs from "node:fs";
export async function connect(port) {
  const sock = net.connect(+port, "127.0.0.1");
  let buf = Buffer.alloc(0), id = 0, hello;
  const waiters = new Map();
  const helloP = new Promise(r => (hello = r));
  sock.on("data", d => {
    buf = Buffer.concat([buf, d]);
    for (;;) {
      const c = buf.indexOf(58);
      if (c < 0) return;
      const n = +buf.subarray(0, c).toString();
      if (buf.length < c + 1 + n) return;
      const msg = JSON.parse(buf.subarray(c + 1, c + 1 + n).toString());
      buf = buf.subarray(c + 1 + n);
      if (!Array.isArray(msg)) { hello(msg); continue; }
      const w = waiters.get(msg[1]); waiters.delete(msg[1]);
      msg[2] ? w.rej(new Error(JSON.stringify(msg[2]))) : w.res(msg[3]);
    }
  });
  const cmd = (name, params = {}) => new Promise((res, rej) => {
    const m = JSON.stringify([0, ++id, name, params]);
    waiters.set(id, { res, rej });
    sock.write(Buffer.byteLength(m) + ":" + m);
  });
  await helloP;
  await cmd("WebDriver:NewSession", { capabilities: { alwaysMatch: { "moz:firefoxOptions": {} } } });
  await cmd("WebDriver:SetTimeouts", { script: 600000, pageLoad: 120000 });
  const ff = {
    cmd,
    go: url => cmd("WebDriver:Navigate", { url }),
    js: async (script, args = []) => (await cmd("WebDriver:ExecuteScript", { script, args })).value,
    ajs: async (script, args = []) => (await cmd("WebDriver:ExecuteAsyncScript", { script, args })).value,
    size: (width, height) => cmd("WebDriver:SetWindowRect", { width, height }),
    shot: async file => { const r = await cmd("WebDriver:TakeScreenshot", { full: false }); fs.writeFileSync(file, Buffer.from(r.value, "base64")); },
    pref: async (name, value) => {
      await cmd("Marionette:SetContext", { value: "chrome" });
      const t = typeof value === "number" ? "Int" : typeof value === "boolean" ? "Bool" : "String";
      await cmd("WebDriver:ExecuteScript", { script: `Services.prefs.set${t}Pref(arguments[0], arguments[1]);`, args: [name, value] });
      await cmd("Marionette:SetContext", { value: "content" });
    },
    /** until polls expr (a JS expression) until it is truthy, and returns its value. */
    until: async (expr, ms = 600000, every = 200) => {
      const t0 = Date.now();
      for (;;) {
        const v = await ff.js(`return (${expr})`);
        if (v) return v;
        if (Date.now() - t0 > ms) throw new Error("timeout waiting for " + expr);
        await new Promise(r => setTimeout(r, every));
      }
    },
    /** frame switches to the iframe that matches the CSS selector, or to the top document with no selector. */
    frame: async sel => {
      if (!sel) return cmd("WebDriver:SwitchToFrame", {});
      const el = await cmd("WebDriver:FindElement", { using: "css selector", value: sel });
      return cmd("WebDriver:SwitchToFrame", { element: Object.values(el.value || el)[0] });
    },
    shotEl: async (sel, file) => {
      const el = await cmd("WebDriver:FindElement", { using: "css selector", value: sel });
      const r = await cmd("WebDriver:TakeScreenshot", { id: Object.values(el.value || el)[0], full: false });
      fs.writeFileSync(file, Buffer.from(r.value, "base64"));
    },
    close: async () => { await cmd("WebDriver:DeleteSession").catch(() => {}); sock.end(); },
  };
  return ff;
}
