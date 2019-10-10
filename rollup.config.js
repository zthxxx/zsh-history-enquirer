import builtins from 'builtin-modules'
import copy from 'rollup-plugin-copy'
import strip from 'rollup-plugin-strip'
import commonjs from 'rollup-plugin-commonjs';
import resolve from 'rollup-plugin-node-resolve';
import filesize from 'rollup-plugin-filesize'
import progress from 'rollup-plugin-progress'

export default {
  input: 'src/index.js',
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
    strip({
      functions: [
        'signale.*',
      ],
      sourceMap: false,
    }),
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
    filesize(),
  ],
}
