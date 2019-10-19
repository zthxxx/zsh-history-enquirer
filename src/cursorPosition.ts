import path from 'path'
import execa from 'execa'


export interface CursorPos {
  x: number,
  y: number,
}

export default async function getCursorPos(cursorScript?: string): Promise<CursorPos> {
  const cursorCommand = cursorScript || path.join(__dirname, 'cursor.zsh')
  const { stdout: position } = await execa(cursorCommand)
  const [row, col] = position.split(' ')
  const [x, y] = [+col, +row]
  return { x, y }
}
