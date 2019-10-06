import builtins from 'builtin-modules'
import copy from 'rollup-plugin-copy'
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
  external: builtins,
  watch: {
    include: 'src',
  },
  plugins: [
    progress(),
    copy({
      targets: [
        { src: 'src/*.zsh', dest: 'dist/' },
      ]
    }),
    resolve({
      jsnext: true,
    }),
    commonjs({
      include: 'node_modules/**',
      sourceMap: false,
    }),
    filesize(),
  ],
}
