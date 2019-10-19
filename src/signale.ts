import fs from 'fs'
import path from 'path'
import Signale from 'signale/signale'


const options = {
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
