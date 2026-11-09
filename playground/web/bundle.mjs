// The bundle format of the playground, for the bundles of the build and the saved workspace of the
// page: the length of the JSON header as a little-endian uint32, the header, then the content of the
// files. header.files lists [path, offset, length, mtimeMs, ...] for each file, with the offset in
// the content.

/**
 * encodeBundle returns a bundle with the fields of header and the files, each [path, data, mtimeMs,
 * ...more]. The row of a file in header.files is [path, offset, length, mtimeMs, ...more].
 */
export function encodeBundle(header, files) {
  const rows = [];
  let contentLength = 0;
  for (const [path, data, mtimeMs, ...more] of files) {
    rows.push([path, contentLength, data.length, mtimeMs, ...more]);
    contentLength += data.length;
  }
  const json = new TextEncoder().encode(JSON.stringify({ ...header, files: rows }));
  const bundle = new Uint8Array(4 + json.length + contentLength);
  new DataView(bundle.buffer).setUint32(0, json.length, true);
  bundle.set(json, 4);
  let offset = 4 + json.length;
  for (const [, data] of files) {
    bundle.set(data, offset);
    offset += data.length;
  }
  return bundle;
}

/** decodeBundle returns the header of bundle and its content, where the offsets of header.files point. */
export function decodeBundle(bundle) {
  const jsonLength = new DataView(bundle.buffer, bundle.byteOffset).getUint32(0, true);
  const header = JSON.parse(new TextDecoder().decode(bundle.subarray(4, 4 + jsonLength)));
  return { header, content: bundle.subarray(4 + jsonLength) };
}
