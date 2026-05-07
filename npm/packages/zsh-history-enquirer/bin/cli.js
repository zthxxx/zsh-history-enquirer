#!/usr/bin/env node
/* eslint-disable no-console */
//
// zsh-history-enquirer — npm shim.
//
// This file does ONE thing: locate the platform-specific Go binary
// installed via optionalDependencies and exec it with the same argv
// the npm bin received. esbuild and biome use the same shape.
//
// We deliberately do not import any third-party module so the shim
// has zero install-time dependency surface beyond Node itself.
//
'use strict';

const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PLUGIN_PATH = path.join(__dirname, '..', 'plugin', 'zsh-history-enquirer.plugin.zsh');

// Map node's platform/arch to the npm sub-package name.
function resolvePlatformPackage() {
  // Go uses "amd64" / "arm64"; node uses "x64" / "arm64". Normalize.
  const archMap = { x64: 'amd64', arm64: 'arm64' };
  const platformMap = { darwin: 'darwin', linux: 'linux' };

  const arch = archMap[process.arch];
  const os = platformMap[process.platform];
  if (!arch || !os) {
    return null;
  }
  return `@zsh-history-enquirer/${os}-${arch}`;
}

function locateBinary() {
  const pkg = resolvePlatformPackage();
  if (!pkg) {
    return null;
  }
  try {
    return require.resolve(`${pkg}/bin/zsh-history-enquirer`);
  } catch (_e) {
    return null;
  }
}

// `npm i` calls us with exactly `--print-install-hint`. Print a
// one-shot reminder telling the user to source the plugin file.
// We never modify the user's .zshrc.
//
// The check is narrowed to `argv.length === 3 && argv[2] === ...`
// (matching the version / help fast-path discipline) so a widget
// invocation `bin -- "$LBUFFER"` whose $LBUFFER literally equals
// "--print-install-hint" cannot trip the fast-path. With the prior
// `argv.includes(...)` check, that argv shape would have printed
// the hint to stderr and exited with empty stdout — silently
// destroying the user's typed text per `BUFFER=$(...)`.
if (
  process.argv.length === 3 &&
  process.argv[2] === '--print-install-hint'
) {
  process.stderr.write([
    '',
    '  zsh-history-enquirer installed.',
    '',
    '  Add this line to your ~/.zshrc to enable the Ctrl+R picker:',
    '',
    `    source ${PLUGIN_PATH}`,
    '',
  ].join('\n') + '\n');
  process.exit(0);
}

// echoArgvAndExit is the BUFFER-preservation fallback for any
// path where the shim cannot complete the picker invocation
// successfully. The widget invokes us inside `BUFFER=$(...)`;
// stdout becomes the user's command line. Any failure that
// leaves stdout empty silently destroys the user's typed text.
//
// The widget passes argv as `cli.js -- "$LBUFFER"`; we strip
// the leading `--` so the echo reflects the user's input rather
// than literally `-- ...`. Always exits 0 because a non-zero
// exit aborts the `$(...)` substitution and likewise loses the
// input.
function echoArgvAndExit() {
  let argv = process.argv.slice(2);
  if (argv[0] === '--') {
    argv = argv.slice(1);
  }
  if (argv.length > 0) {
    process.stdout.write(argv.join(' ') + '\n');
  }
  process.exit(0);
}

const bin = locateBinary();
if (!bin) {
  // No platform-specific binary installed (e.g. a platform that
  // wasn't included in optionalDependencies resolution).
  process.stderr.write(
    'zsh-history-enquirer: no platform binary installed for ' +
      `${process.platform}-${process.arch}.\n` +
    'Install one of @zsh-history-enquirer/<os>-<arch> manually if\n' +
    'your platform was excluded from optionalDependencies resolution.\n'
  );
  echoArgvAndExit();
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });

// spawnSync sets `result.error` when the child could not be
// spawned at all (ENOENT, EACCES, ETXTBSY, etc.) — distinct from
// a child that ran but exited non-zero. require.resolve above
// only confirmed file existence; a binary that lost its +x bit
// (some Docker mounts, a botched npm-cache extraction, a stale
// symlink) would slip past locateBinary and fail here. Without
// the same echo-argv fallback, BUFFER=$(...) would blank the
// user's typed text on every Ctrl-R.
if (result.error) {
  process.stderr.write(
    'zsh-history-enquirer: failed to exec platform binary ' +
      `(${bin}): ${result.error.code || result.error.message}\n`
  );
  echoArgvAndExit();
}

// result.status is null when the child was killed by a signal —
// preserve the widget contract by exiting 0 in that case so
// $(...) doesn't abort. The picker's own signal handlers leave
// stdout written if it had emitted a result before the signal,
// so this path is rarely user-visible.
process.exit(result.status === null ? 0 : result.status);
