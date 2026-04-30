import globals from 'globals';
import pluginJs from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactX from 'eslint-plugin-react-x';
import pluginReactConfig from 'eslint-plugin-react/configs/recommended.js';
import eslintPluginPrettierRecommended from 'eslint-plugin-prettier/recommended';

export default [
    {languageOptions: {globals: globals.browser}},
    pluginJs.configs.recommended,
    ...tseslint.configs.recommended,
    {
        rules: {
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/ban-types': 'off',
            '@typescript-eslint/no-var-requires': 'off'
        }
    },
    {
        ...reactX.configs.recommended,
        rules: {
            // ...reactX.configs.strict.rules,
            'react-x/no-class-component': 'error'
        }
    },
    {
        settings: {
            react: {
                version: 'detect'
            }
        },
        ...pluginReactConfig,
        rules: {
            // TODO: Re-enable these rules that were disabled by mistake
            // ...pluginReactConfig.rules,
            'react/display-name': 'off',
            'react/no-string-refs': 'off',
            'react/prefer-stateless-function': 'error',
            'react/jsx-no-useless-fragment': ['error', {allowExpressions: true}]
        }
    },
    eslintPluginPrettierRecommended,
    {
        files: ['./src/**/*.{ts,tsx}']
    },
    {
        ignores: ['dist', 'assets', '**/*.config.js', 'jest.setup.js', '__mocks__', 'coverage', '**/*.test.{ts,tsx}']
    }
];
