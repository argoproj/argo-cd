'use strict;';

const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const GoogleFontsPlugin = require('@beyonk/google-fonts-webpack-plugin');
const webpack = require('webpack');
const path = require('path');

const isProd = process.env.NODE_ENV === 'production';
console.log(`Bundling in ${isProd ? 'production' : 'development'} mode...`);

const proxyConf = {
    'target': process.env.ARGOCD_API_URL || 'http://localhost:8080',
    'secure': false,
};

const config = {
    entry: './src/app/index.tsx',
    output: {
        filename: '[name].[hash].js',
        chunkFilename: '[name].[hash].chunk.js',
        path: __dirname + '/../../dist/app'
    },

    devtool: 'source-map',

    resolve: {
        extensions: ['.ts', '.tsx', '.js', '.json']
    },

    module: {
        rules: [
            {
                test: /\.tsx?$/,
                loaders: [ ...( isProd ? [] : ['react-hot-loader/webpack']), `ts-loader?allowTsInNodeModules=true&configFile=${path.resolve('./src/app/tsconfig.json')}`]
            }, {
                enforce: 'pre',
                exclude: [
                    /node_modules\/react-paginate/,
                    /node_modules\/monaco-editor/,
                ],
                test: /\.js$/,
                loaders: [...(isProd ? ['babel-loader'] : []), 'source-map-loader'],
            }, {
                test: /\.scss$/,
                loader: 'style-loader!raw-loader!sass-loader'
            }, {
                test: /\.css$/,
                loader: 'style-loader!raw-loader'
            },
        ]
    },
    node: {
        fs: 'empty',
    },
    plugins: [
        new webpack.DefinePlugin({
            'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV || 'development'),
            SYSTEM_INFO: JSON.stringify({
                version: process.env.ARGO_VERSION || 'latest',
            }),
        }),
        new HtmlWebpackPlugin({ template: 'src/app/index.html' }),
        new CopyWebpackPlugin({
            patterns: [{
                from: 'src/assets', to: 'assets'
            }, {
                from: 'node_modules/argo-ui/src/assets', to: 'assets'
            }, {
                from: 'node_modules/@fortawesome/fontawesome-free/webfonts', to: 'assets/fonts'
            }, {
                from: 'node_modules/redoc/bundles/redoc.standalone.js', to: 'assets/scripts/redoc.standalone.js'
            }]
        }),
        new MonacoWebpackPlugin({
            // https://github.com/microsoft/monaco-editor-webpack-plugin#options
            languages: [ 'yaml' ]
        }),
        new GoogleFontsPlugin({
            // config: https://github.com/beyonk-adventures/google-fonts-webpack-plugin
            // the upstream version of this plugin is not compatible with webpack 4 so we use this fork
			fonts: [{
                family: 'Heebo',
                variants: [ '300', '400', '500', '700' ]
            }],
            // This works by downloading the fonts at bundle time and adding those font-faces to 'fonts.css'.
            name: 'fonts',
            filename: 'fonts.css',
            // local: false in dev prevents pulling fonts on each code change
            // https://github.com/gabiseabra/google-fonts-webpack-plugin/issues/2
            local: isProd,
            path: 'assets/fonts/google-fonts'
		})
    ],
    devServer: {
        historyApiFallback: {
            disableDotRule: true
        },
        port: 4000,
        host: process.env.ARGOCD_E2E_YARN_HOST || 'localhost',
        proxy: {
            '/api': proxyConf,
            '/auth': proxyConf,
            '/swagger-ui': proxyConf,
            '/swagger.json': proxyConf,
        }
    }
};

module.exports = config;
