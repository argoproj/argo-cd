module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'jsdom',
  reporters: ['default', 'jest-junit'],
  collectCoverage: true,
  transformIgnorePatterns: ['node_modules/(?!(argo-ui)/)'],
  globals: {
    'self': {},
    'ts-jest': {
      isolatedModules: true,
    },
  },
  moduleNameMapper: {
    // https://github.com/facebook/jest/issues/3094
    '\\.(jpg|jpeg|png|gif|eot|otf|webp|svg|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga)$': '<rootDir>/__mocks__/fileMock.js',
    '.+\\.(css|styl|less|sass|scss)$': 'jest-transform-css',
  },
};
