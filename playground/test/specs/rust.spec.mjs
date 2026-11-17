// The Rust crate of the sample: the markers that the Rust helper serves run in the tab, and the
// markers that need cargo are errors that name the marker.
import { test, expect, RUN_MS } from "../fixtures.mjs";

const LIB = "crates/lamp/src/lib.rs";

test("docgen update reads an edit of the Rust crate into the page", async ({ playground }) => {
  expect(await playground.exists("docs/rust.md")).toBe(true);
  expect(await playground.replace(LIB, "pub const MAX_LEVEL: u8 = 100;", "pub const MAX_LEVEL: u8 = 90;")).toBe(true);
  expect(await playground.replace(LIB, "/// Sets the level of the lamp.", "/// Sets the level of the lamp at once.")).toBe(
    true,
  );
  const update = await playground.runButton("update");
  expect(update.code, update.status).toBe(0);
  expect(update.changed).toContain("dg/docs/rust.md");
  const text = await playground.text("docs/rust.md");
  expect(text).toContain("name=MAX_LEVEL print=raw -->`90`<");
  expect(text).toContain("name=STEP print=raw -->`MAX_LEVEL / 4`<");
  expect(text).toContain("Sets the level of the lamp at once.");
  expect(text).toContain("A dimmable lamp, the Rust twin of the Go package machine of this sample.");
  expect(text).toContain("```rust\npub fn set(&mut self, level: u8) -> Result<(), LampError> {");
  expect(text).toMatch(/\| `LampError::TooBright` \| `Lamp::set` \| `level \{level\} is above the highest level` \|/);
  expect(text).toMatch(/\| `LampError::Broken` \| `Lamp::brighter`, `Lamp::set` \| `the lamp is broken` \|/);
});

test("the shell runs the Rust helper as a WASI process with its exit status", async ({ playground }) => {
  const read = await playground.runCommand("docgen-rs-helper --symbols /work/dg/crates/lamp/src/lib.rs");
  expect(read.code, read.status).toBe(0);
  expect(await playground.output()).toContain('"kind":"fn","line":26,"name":"Lamp::set"');
  const usage = await playground.runCommand("docgen-rs-helper; echo status=$?");
  expect(usage.code, usage.status).toBe(0);
  const output = await playground.output();
  expect(output).toContain("usage: docgen-rs-helper [--symbols] <file>...");
  expect(output).toContain("status=2");
});

test("a Rust marker that needs cargo is an error that names it", async ({ playground }) => {
  test.setTimeout(2 * RUN_MS);
  const page = "docs/rust.md";
  expect(await playground.replace(page, "name=STEP print=raw -->", "name=STEP -->")).toBe(true);
  expect(
    await playground.replace(
      page,
      "## Set\n",
      "## Set\n\n<!-- docgen:output crate=lamp doctest=Lamp::set -->\n<!-- /docgen:output -->\n",
    ),
  ).toBe(true);
  const check = await playground.runButton("check");
  expect(check.code, check.status).toBe(1);
  const output = await playground.output(4000);
  expect(output).toMatch(
    /docs\/rust\.md:\d+: docgen:const crate=lamp name=STEP: this marker needs cargo, and the playground has no Rust toolchain/,
  );
  expect(output).toMatch(
    /docs\/rust\.md:\d+: docgen:output crate=lamp doctest=Lamp::set: this marker needs cargo, and the playground has no Rust toolchain/,
  );
  expect(output).not.toMatch(/docgen:const crate=lamp name=MAX_LEVEL print=raw:/);
  expect(output).toContain("cargo is not on PATH, so docgen reads the Cargo workspace from its Cargo.toml files");
});
