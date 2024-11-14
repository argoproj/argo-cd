import globals from 'globals';
import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactPlugin from 'eslint-plugin-react';
import prettierConfig from 'eslint-plugin-prettier/recommended';

export default [
    {
        languageOptions: {
            // Enable the browser global variables
            globals: globals.browser
        }
    },

    // Recommended ESLint configuration
    eslint.configs.recommended,

    // Recommended TypeScript configuration
    ...tseslint.configs.recommended,

    // React configuration
    {
        files: ['**/*.jsx', '**/*.tsx'],
        ...reactPlugin.configs.flat.recommended,
        languageOptions: {
            ...reactPlugin.configs.flat.recommended.languageOptions
        },
        settings: {
            react: {
                version: 'detect'
            }
        }
    },

    // Prettier configuration to disable conflicting ESLint rules
    prettierConfig,

    // Disable rules
    {
        rules: {
            // ESLint
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/ban-types': 'off',
            '@typescript-eslint/no-var-requires': 'off',

            // React
            'react/display-name': 'off',
            'react/no-string-refs': 'off',

            // TODO: remove these after fixing the corresponding issues
            'react/jsx-key': 'off',
            'react/jsx-no-target-blank': 'off',
            'react/no-children-prop': 'off',
            'react/no-unescaped-entities': 'off',
            'react/no-unknown-property': 'off',
            'react/no-deprecated': 'off'
        }
    },

    // Global Ignore
    {
        ignores: [
            // Files
            '**/*.config.js',
            '**/*.test.{ts,tsx}',

            // Directories
            '**/__mocks__/',
            '**/assets/',
            '**/coverage/',
            '**/dist/',
            '**/node_modules/'
        ]
    }
];
