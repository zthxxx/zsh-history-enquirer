import fs from 'fs'
import tty from 'tty'


export const getStdin = async () => new tty.ReadStream(fs.openSync('/dev/tty', 'r'))
export const getStdout = async () => new tty.WriteStream(fs.openSync('/dev/tty', 'w'))
