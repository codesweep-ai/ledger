#!/usr/bin/env node
// Viewer-asset test suite — plain node, no framework.
// The validator/renderer suite lives in Go (internal/ledger); this file tests
// the browser-side code that ships inside ledger.html: the markdown renderer
// in viewer/viewer.js, loaded from the real asset so what's tested is what
// ships.
// Usage: node test/run.mjs
import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';

let passed = 0, failed = 0;
function t(name, fn) {
  try { fn(); passed++; console.log('  ok  ' + name); }
  catch (e) { failed++; console.error('FAIL  ' + name + '\n      ' + (e.message || e).split('\n')[0]); }
}

const viewerSrc = readFileSync(new URL('../viewer/viewer.js', import.meta.url), 'utf8');
const mdToHtml = new Function(
  viewerSrc.slice(0, viewerSrc.indexOf('\n(function viewerMain(')) + '\nreturn mdToHtml;'
)();

console.log('markdown (viewer/viewer.js):');
t('html in markdown is escaped', () => {
  const out = mdToHtml('hello <script>alert(1)</script> & <img src=x onerror=y>');
  assert.doesNotMatch(out, /<script>|<img/);
  assert.match(out, /&lt;script&gt;/);
});
t('fences, inline code, bold, tables render', () => {
  const out = mdToHtml('# T\n\n**bold** and `code`\n\n```js\nconst x = 1 < 2;\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n');
  assert.match(out, /<h3>T<\/h3>/);
  assert.match(out, /<strong>bold<\/strong>/);
  assert.match(out, /<code class="mdi">code<\/code>/);
  assert.match(out, /<pre class="mdcode"><code>const x = 1 &lt; 2;<\/code><\/pre>/);
  assert.match(out, /<table class="mdtbl">[\s\S]*<th>a<\/th>[\s\S]*<td>2<\/td>/);
});
t('lists render, multi-line items fold', () => {
  const out = mdToHtml('- one\n- two\n  continued\n\n1. first\n2. second\n');
  assert.match(out, /<ul><li>one<\/li><li>two continued<\/li><\/ul>/);
  assert.match(out, /<ol><li>first<\/li><li>second<\/li><\/ol>/);
});
t('only http(s) and #fragment links become anchors', () => {
  const ok = mdToHtml('[good](https://example.com) [frag](#TST-001)');
  assert.match(ok, /href="https:\/\/example.com"/);
  assert.match(ok, /href="#TST-001"/);
  const bad = mdToHtml('[evil](javascript:alert(1))');
  assert.doesNotMatch(bad, /href/);
});

console.log('\n' + passed + ' passed, ' + failed + ' failed');
process.exit(failed ? 1 : 0);
