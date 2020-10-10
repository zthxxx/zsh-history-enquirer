// https://rollupjs.org/guide/en/#configuration-files

import builtins from 'builtin-modules'
import json from '@rollup/plugin-json'
import commonjs from '@rollup/plugin-commonjs'
import resolve from '@rollup/plugin-node-resolve'
import strip from '@rollup/plugin-strip'
import typescript from 'rollup-plugin-typescript'
import copy from 'rollup-plugin-copy'
import filesize from 'rollup-plugin-filesize'
import progress from 'rollup-plugin-progress'
import packageJson from './package.json'


export default {
  input: 'src/index.ts',
  output: {
    file: 'dist/index.js',
    format: 'cjs',
    exports: 'auto',
  },
  watch: {
    include: 'src',
  },
  external: [
    ...builtins,
    ...Object.keys(packageJson.dependencies ?? {}),
    'signale/signale',
  ],
  treeshake: {
    moduleSideEffects: false,
  },
  plugins: [
    progress(),
    copy({
      targets: [
        { src: 'src/*.zsh', dest: 'dist/' },
      ]
    }),
    resolve(),
    json(),
    commonjs({
      include: 'node_modules/**',
      sourceMap: false,
    }),
    typescript(),
    strip({
      include: [
        '**/*.js',
        '**/*.ts',
      ],
      functions: [
        'signale.*',
      ],
    }),
    filesize(),
  ],
}
