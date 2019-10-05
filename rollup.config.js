import copy from 'rollup-plugin-copy'

export default {
  input: 'src/index.js',
  output: {
    file: 'dist/index.js',
    format: 'cjs',
  },
  watch: {
    include: 'src',
  },
  plugins: [
    copy({
      targets: [
        { src: 'src/*.zsh', dest: 'dist/' },
      ]
    })
  ]
}
