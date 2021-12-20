module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  reporters: ['default', 'jest-junit'],
  collectCoverage: true,
  transformIgnorePatterns: ['node_modules/(?!(argo-ui)/)'],
  globals: {
    'window': {localStorage: { getItem: () => '{}', setItem: () => null }},
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

const localStorageMock = (() => {
  let store = {};
  return {
    getItem: (key) => store[key],
    setItem: (key, value) => {
      store[key] = value.toString();
    },
    clear: () => {
      store = {};
    },
    removeItem: (key) => {
      delete store[key];
    }
  };
})();
global.localStorage = localStorageMock;