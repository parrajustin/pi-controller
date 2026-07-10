export default {
  testPathIgnorePatterns: ['/node_modules/', '/e2e/'],
  preset: 'ts-jest',
  testEnvironment: 'jest-environment-jsdom',
  testEnvironmentOptions: {
    url: "http://localhost/"
  },
  transform: {
    '^.+\\.[tj]sx?$': ['ts-jest', {
      tsconfig: { allowJs: true, esModuleInterop: true },
      isolatedModules: true
    }],
  },
  transformIgnorePatterns: [
    "node_modules/(?!.*(@material|lit|@lit|standard-ts-lib|date-fns|qrcode|element-internals-polyfill))"
  ],
  moduleNameMapper: {
    '^(\\.{1,2}/.*)\\.js$': '$1',
    '^standard-ts-lib/(.*)\\.js$': 'standard-ts-lib/$1.ts',
    '\\.(png|jpg|jpeg|gif|webp|svg)$': '<rootDir>/__mocks__/fileMock.js',
  },
  extensionsToTreatAsEsm: ['.ts'],
  setupFilesAfterEnv: ['<rootDir>/src/setupTests.ts'],
};
