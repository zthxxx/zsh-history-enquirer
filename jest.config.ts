import type { Config } from '@jest/types'


const config: Config.InitialOptions = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  testRegex: '\\.test\\.ts$',
  collectCoverage: true,
  collectCoverageFrom: [
    "src/*.{js,ts}",
    "!**/node_modules/**",
    "!**/dist/**",
    "!**/scripts/**",
  ],

  globals: {
    'ts-jest': {
      isolatedModules: true,
    },
  },
}

export default config
