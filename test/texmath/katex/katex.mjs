// The KaTeX oracle of texmath (D78).
//
// The script reads a corpus file: one formula on each line, with blank lines
// and lines that start with "#" left out. For each formula it writes one line
// of JSON to the standard output, in the order of the corpus, after a first
// line with the version of KaTeX:
//
//   {"version": "0.18.11"}
//   {"tex": "...", "ok": true, "height": 1.2, "depth": 0.4}
//   {"tex": "...", "ok": false, "crash": false, "error": "KaTeX parse error: ..."}
//
// The height and the depth are in em, from the tree of boxes that KaTeX
// builds. KaTeX in Node does not draw, so it gives no width. The options are
// the options of Mermaid 12.3.0: displayMode true and throwOnError true. The
// option strict is "ignore", so that KaTeX writes no warning. A warning of
// the default "warn" does not change the result.
//
// With the flag --html after the corpus, each formula that parses also has
// the field "html", the HTML of KaTeX with the same options, for the report
// of Chromium.
//
// Run it as: node katex.mjs CORPUS [--html]

import { readFileSync } from "node:fs";
import katex from "katex";

const corpus = readFileSync(process.argv[2], "utf8");
const withHTML = process.argv[3] === "--html";
const options = {
  displayMode: true,
  throwOnError: true,
  strict: "ignore",
};
const out = [JSON.stringify({ version: katex.version })];
for (const line of corpus.split("\n")) {
  if (line.trim() === "" || line.startsWith("#")) {
    continue;
  }
  try {
    const tree = katex.__renderToHTMLTree(line, options);
    const result = { tex: line, ok: true, height: tree.height, depth: tree.depth };
    if (withHTML) {
      result.html = katex.renderToString(line, options);
    }
    out.push(JSON.stringify(result));
  } catch (err) {
    // An error that is not a ParseError is a fault of KaTeX, such as a
    // TypeError on some malformed macros. Mermaid shows no formula then
    // either, so the formula does not parse.
    const crash = !(err instanceof katex.ParseError);
    out.push(JSON.stringify({ tex: line, ok: false, crash, error: String(err.message) }));
  }
}
out.push("");
process.stdout.write(out.join("\n"));
