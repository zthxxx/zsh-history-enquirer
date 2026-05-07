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

// `npm i` calls us with `--print-install-hint`. Print a one-shot
// reminder telling the user to source the plugin file. We never
// modify the user's .zshrc.
if (process.argv.includes('--print-install-hint')) {
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

const bin = locateBinary();
if (!bin) {
  // Print the diagnostic to stderr so it appears at the user's
  // terminal (the widget's BUFFER=$(...) only captures stdout).
  process.stderr.write(
    'zsh-history-enquirer: no platform binary installed for ' +
      `${process.platform}-${process.arch}.\n` +
    'Install one of @zsh-history-enquirer/<os>-<arch> manually if\n' +
    'your platform was excluded from optionalDependencies resolution.\n'
  );

  // CRITICAL: echo argv back to stdout so the widget's
  // `BUFFER=$(...)` does not blank the user's typed line. The
  // widget contract is "stdout must reproduce the input on any
  // failure path"; without this echo, a missing platform binary
  // silently eats the user's keystrokes.
  //
  // The widget invokes us as `cli.js -- "$LBUFFER"` (the `--`
  // protects $LBUFFER strings that look like flags from triggering
  // the binary's --version / --help fast-path). We must strip a
  // leading `--` here too, otherwise `BUFFER=$(...)` lands as
  // `-- $LBUFFER` instead of the user's actual typed text.
  let argv = process.argv.slice(2);
  if (argv[0] === '--') {
    argv = argv.slice(1);
  }
  if (argv.length > 0) {
    process.stdout.write(argv.join(' ') + '\n');
  }

  // Exit 0 so `BUFFER=$(...)` doesn't abort. The plugin file's
  // graceful native-^R fallback only kicks in when the binary
  // isn't on $PATH at all; once npm has placed cli.js there, it
  // is on $PATH but unable to do its job, so this echo path is
  // the next-best UX.
  process.exit(0);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
process.exit(result.status === null ? 0 : result.status);
