// cli.test.js — node:test smoke checks for the npm shim.
//
// We do NOT pull in jest / mocha / etc. — node:test is part of node
// 18+ and the project's stated minimum (see package.json "engines"),
// so a built-in test runner keeps the install-time dependency surface
// at zero. Run with: `node --test bin/cli.test.js`.
//
// The tests spawn cli.js as a child process so we exercise the real
// argv-handling path (process.argv shape, stdout / stderr split).
// They do NOT require any platform binary to be installed — the no-
// binary fallback path is itself part of what we want to pin.

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const CLI = path.join(__dirname, 'cli.js');

function run(args) {
  const r = spawnSync(process.execPath, [CLI, ...args], {
    encoding: 'utf8',
  });
  return {
    status: r.status,
    stdout: r.stdout || '',
    stderr: r.stderr || '',
  };
}

// The bare install-hint fast-path: npm's postinstall hook invokes
// the shim with exactly `--print-install-hint`. Hint goes to stderr
// (so it surfaces at the user's terminal rather than being captured
// by anything reading stdout), and stdout stays empty.
test('bare --print-install-hint prints hint to stderr', () => {
  const r = run(['--print-install-hint']);
  assert.equal(r.status, 0);
  assert.match(r.stderr, /zsh-history-enquirer installed/);
  assert.match(r.stderr, /source /);
  assert.equal(r.stdout, '', 'stdout must stay empty for the hint path');
});

// The widget contract: a $LBUFFER of "--print-install-hint" must NOT
// trigger the install-hint fast-path. The widget invokes the shim as
// `bin -- "$LBUFFER"`, so the argv shape is `[..., '--', '--print-install-hint']`.
// With the prior `argv.includes(...)` check, this would have printed
// the hint to stderr, exited 0, and left stdout empty — silently
// destroying the user's typed text per `BUFFER=$(...)`.
test('widget mode with --print-install-hint as LBUFFER preserves input', () => {
  const r = run(['--', '--print-install-hint']);
  assert.equal(r.status, 0);
  // The fallback path echoes the post-`--` argv to stdout.
  assert.equal(
    r.stdout,
    '--print-install-hint\n',
    'stdout must echo the typed text — widget contract',
  );
  // stderr will likely include the no-platform-binary diagnostic in
  // dev / unbuilt environments. The important thing is the install
  // hint specifically must NOT appear there.
  assert.doesNotMatch(
    r.stderr,
    /zsh-history-enquirer installed/,
    'install hint must not surface on widget-mode invocations',
  );
});

// Symmetric guard: a single positional arg that happens to be
// "--print-install-hint" with no `--` separator must still work like
// the postinstall path. This keeps the narrowed check from being
// too picky.
test('argv length 3 + bare --print-install-hint still triggers hint', () => {
  // process.argv inside cli.js will be [node, cli.js, --print-install-hint]
  // i.e. length 3, argv[2] === '--print-install-hint'.
  const r = run(['--print-install-hint']);
  assert.equal(r.status, 0);
  assert.match(r.stderr, /zsh-history-enquirer installed/);
});
