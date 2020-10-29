import fs from 'fs'
import path from 'path'
import Signale from 'signale/signale'


/** https://github.com/klaussinani/signale#custom-loggers */
const options = {
  /** https://github.com/klaussinani/signale#loglevel */
  logLevel: 'debug',

  /** https://github.com/klaussinani/signale#stream */
  stream: [
    fs.createWriteStream(
      path.join(__dirname, '../debug.log'),
      {
        flags: 'a',
      },
    ),
  ],
}

export default new Signale(options)
