import builtins from 'builtin-modules'
import copy from 'rollup-plugin-copy'
import strip from '@rollup/plugin-strip'
import typescript from 'rollup-plugin-typescript'
import commonjs from 'rollup-plugin-commonjs'
import resolve from 'rollup-plugin-node-resolve'
import filesize from 'rollup-plugin-filesize'
import progress from 'rollup-plugin-progress'

export default {
  input: 'src/index.ts',
  output: {
    file: 'dist/index.js',
    format: 'cjs',
  },
  watch: {
    include: 'src',
  },
  external: [
    ...builtins,
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
