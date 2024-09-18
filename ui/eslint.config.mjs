import globals from 'globals';
import pluginJs from '@eslint/js';
import tseslint from 'typescript-eslint';
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
        settings: {
            react: {
                version: 'detect'
            }
        },
        ...pluginReactConfig,
        rules: {
            'react/display-name': 'off',
            'react/no-string-refs': 'off'
        }
    },
    eslintPluginPrettierRecommended,
    {
        files: ['./src/**/*.{ts,tsx}']
    },
    {
        ignores: ['dist', 'assets', '**/*.config.js', '__mocks__', 'coverage', '**/*.test.{ts,tsx}']
    }
];
