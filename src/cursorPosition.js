import path from 'path'
import execa from 'execa'


export default async function getCursorPos(cursorScript) {
  const cursorCommand = cursorScript || path.join(__dirname, 'cursor.zsh')
  const { stdout: position } = await execa(cursorCommand)
  const [row, col] = position.split(' ')
  const [x, y] = [+col, +row]
  return { x, y }
}
