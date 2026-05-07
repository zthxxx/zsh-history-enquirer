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
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const CLI = path.join(__dirname, 'cli.js');

function run(args, env) {
  const r = spawnSync(process.execPath, [CLI, ...args], {
    encoding: 'utf8',
    env: env ? { ...process.env, ...env } : process.env,
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
  // The plugin path must be wrapped in double quotes so a copy-paste
  // survives install prefixes that contain spaces (e.g. macOS users
  // whose home directory has a space).
  assert.match(
    r.stderr,
    /source "[^"]+\.plugin\.zsh"/,
    'plugin path must be double-quoted in the hint',
  );
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

// BUFFER-preservation contract on the no-platform-binary fallback:
// the shim is invoked as `cli.js -- "$LBUFFER"` by the zsh widget,
// and the binary may be missing in dev / unbuilt / cross-platform
// environments. The shim must echo the typed text to stdout so
// $(...) lands as the user's command line, not as empty BUFFER.
test('no-binary fallback echoes argv to stdout (widget shape)', () => {
  const r = run(['--', 'git status']);
  assert.equal(r.status, 0);
  assert.equal(
    r.stdout,
    'git status\n',
    'stdout must echo the user-typed text — widget contract',
  );
});

// Symmetric guard: empty argv on the fallback path should NOT
// emit a stray newline that would land as a single LF in BUFFER.
test('no-binary fallback with no argv emits empty stdout', () => {
  const r = run([]);
  assert.equal(r.status, 0);
  assert.equal(r.stdout, '', 'no argv → no stdout (don\'t blank BUFFER with a stray \\n)');
});

// The exec-failure path: locateBinary may succeed (the file
// exists) but spawnSync may then fail because the binary lost
// its executable bit, the path is a stale symlink, or some other
// FS-level error. Without the result.error fallback, the shim
// would exit 0 with empty stdout — silently destroying the
// user's typed text on every Ctrl-R.
//
// We reproduce the scenario by building a fake platform-package
// next to a copy of cli.js and pointing NODE_PATH at it. The
// fake package contains a `bin/zsh-history-enquirer` file that
// exists (so require.resolve in locateBinary succeeds) but is
// chmod 0644 (so spawnSync fails with EACCES).
test('exec-failure (non-executable binary) echoes argv to stdout', { skip: process.platform === 'win32' }, () => {
  const arch = { x64: 'amd64', arm64: 'arm64' }[process.arch];
  const platform = { darwin: 'darwin', linux: 'linux' }[process.platform];
  if (!arch || !platform) {
    return; // unsupported platform; cli.js wouldn't even try.
  }

  // tmp/
  //   node_modules/
  //     @zsh-history-enquirer/<platform>-<arch>/
  //       package.json  (so require.resolve walks up cleanly)
  //       bin/zsh-history-enquirer  (regular file, mode 0644)
  //   shim/
  //     cli.js  (copy of the real shim)
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'zhe-shim-test-'));
  try {
    const pkgDir = path.join(tmp, 'node_modules', '@zsh-history-enquirer', `${platform}-${arch}`);
    fs.mkdirSync(path.join(pkgDir, 'bin'), { recursive: true });
    fs.writeFileSync(
      path.join(pkgDir, 'package.json'),
      JSON.stringify({ name: `@zsh-history-enquirer/${platform}-${arch}`, version: '0.0.0' }),
    );
    fs.writeFileSync(path.join(pkgDir, 'bin', 'zsh-history-enquirer'), '#!/bin/false\n', { mode: 0o644 });

    const shimDir = path.join(tmp, 'shim');
    fs.mkdirSync(shimDir);
    const shimPath = path.join(shimDir, 'cli.js');
    fs.copyFileSync(CLI, shimPath);

    const r = spawnSync(process.execPath, [shimPath, '--', 'git status'], {
      encoding: 'utf8',
      env: { ...process.env, NODE_PATH: path.join(tmp, 'node_modules') },
    });

    assert.equal(r.status, 0, 'exit must be 0 to keep $(...) substitution alive');
    assert.equal(
      r.stdout,
      'git status\n',
      'stdout must echo the typed text — widget contract on exec failure',
    );
    assert.match(
      r.stderr || '',
      /failed to exec platform binary/,
      'stderr should explain why the picker did not open',
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
