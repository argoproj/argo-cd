'use strict;';

const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const {codecovWebpackPlugin} = require("@codecov/webpack-plugin");
const webpack = require('webpack');

const isProd = process.env.NODE_ENV === 'production';

console.log(`Bundling in ${isProd ? 'production' : 'development'}...`);

const proxyConf = {
    target: process.env.ARGOCD_API_URL || 'http://localhost:8080',
    secure: false,
    // Rewrite Host header when proxying to a remote API server (e.g. a hosted Argo CD instance).
    changeOrigin: !!process.env.ARGOCD_API_URL
};

const config = {
    entry: './src/app/index.tsx',
    output: {
        filename: '[name].[contenthash].js',
        chunkFilename: '[name].[contenthash].chunk.js',
        path: __dirname + '/../../dist/app',
        clean: true
    },
    cache: { type: 'filesystem' },

    resolve: {
        extensions: ['.ts', '.tsx', '.js', '.json'],
        alias: {
            'react-form': require.resolve('argo-ui/src/components/form/compat.tsx'),
        },
        fallback: { fs: false }
    },
    ignoreWarnings: [{
        module: new RegExp('/node_modules/argo-ui/.*')
    }],
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                loader: 'esbuild-loader',
                options: {
                    loader: 'tsx',
                    target: 'es2015',
                    tsconfigRaw: require('./tsconfig.json')
                }
            },
            {
                enforce: 'pre',
                test: /\.js$/,
                exclude: [/node_modules\/react-paginate/, /node_modules\/monaco-editor/],
                use: ['source-map-loader'],
            },
            {
                enforce: 'pre',
                exclude: [/node_modules\/react-paginate/, /node_modules\/monaco-editor/],
                test: /\.js$/,
                use: ['esbuild-loader'],
            },
            {
                test: /\.scss$/,
                use: [
                    'style-loader',
                    {
                        loader: 'css-loader',
                        options: { url: false, import: false }
                    },
                    {
                        loader: 'sass-loader',
                        options: {
                            sassOptions: {
                                includePaths: ['node_modules'],
                                quietDeps: true,
                                silenceDeprecations: ['import', 'legacy-js-api', 'global-builtin', 'color-functions']
                            }
                        }
                    }
                ]
            },
            {
                test: /\.css$/,
                use: [
                    'style-loader',
                    {
                        loader: 'css-loader',
                        options: { url: false, import: false }
                    }
                ]
            }
        ]
    },
    plugins: [
        new webpack.DefinePlugin({
            'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV || 'development'),
            'process.env.NODE_ONLINE_ENV': JSON.stringify(process.env.NODE_ONLINE_ENV || 'offline'),
            'process.platform': JSON.stringify('browser'),
            'SYSTEM_INFO': JSON.stringify({
                version: process.env.ARGO_VERSION || 'latest'
            })
        }),
        new HtmlWebpackPlugin({ template: 'src/app/index.html' }),
        new CopyWebpackPlugin({
            patterns: [{
                    from: 'src/assets',
                    to: 'assets'
                },
                {
                    from: 'node_modules/argo-ui/src/assets',
                    to: 'assets'
                },
                {
                    from: 'node_modules/@fortawesome/fontawesome-free/webfonts',
                    to: 'assets/fonts'
                },
                {
                    from: 'node_modules/redoc/bundles/redoc.standalone.js',
                    to: 'assets/scripts/redoc.standalone.js'
                },
                {
                    from: 'node_modules/monaco-editor/min/vs/base/browser/ui/codicons/codicon',
                    to: 'assets/fonts'
                }
            ]
        }),
        new MonacoWebpackPlugin({
            // https://github.com/microsoft/monaco-editor-webpack-plugin#options
            languages: ['yaml']
        }),
        codecovWebpackPlugin({
            enableBundleAnalysis: process.env.CODECOV_TOKEN !== undefined,
            bundleName: "argo-cd-ui",
            uploadToken: process.env.CODECOV_TOKEN,
        }),
    ],
    devServer: {
        compress: false,
        historyApiFallback: {
            disableDotRule: true
        },
        port: 4000,
        host: process.env.ARGOCD_E2E_JS_HOST || 'localhost',
        client: {
            overlay: {
                errors: true,
                warnings: false,
                // Suppress 401 ResponseError overlays — the onError stream
                // in app.tsx already handles them by redirecting to login.
                runtimeErrors: (error) => {
                    if (error.message && error.message.includes('Unauthorized')) {
                        return false;
                    }
                    if (error.message && error.message.includes('401')) {
                        return false;
                    }
                    return !(error && error.name === 'ResponseError' && error.status === 401);
                }
            }
        },
        proxy: [
            {
                context: ['/extensions', '/api', '/auth', '/swagger-ui', '/swagger.json', '/download'],
                ...proxyConf
            },
            {
                context: ['/terminal'],
                target: process.env.ARGOCD_API_URL || 'ws://localhost:8080',
                ws: true,
            }
        ]
    }
};

if (isProd) {
    config.performance = {
        hints: 'error',
        // Max size is 6MB before gzip.
        maxEntrypointSize: 6 * 1024 * 1024,
        maxAssetSize: 6 * 1024 * 1024,
    };
}

config.devtool = isProd ? 'source-map' : 'eval-source-map';

module.exports = config;
