'use strict;';

const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const webpack = require('webpack');
const path = require('path');

const isProd = process.env.NODE_ENV === 'production';

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
                loaders: [ ...( isProd ? ['babel-loader?presets=babel-preset-env'] : []), 'source-map-loader']
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
        new CopyWebpackPlugin([{
            from: 'src/assets', to: 'assets'
        }, {
            from: 'node_modules/argo-ui/src/assets', to: 'assets'
        }, {
            from: 'node_modules/@fortawesome/fontawesome-free/webfonts', to: 'assets/fonts'
        }]),
        new MonacoWebpackPlugin(),
    ],
    devServer: {
        historyApiFallback: true,
        port: 4000,
        proxy: {
            '/api': proxyConf,
            '/auth': proxyConf,
            '/swagger-ui': proxyConf,
            '/swagger.json': proxyConf,
        }
    }
};

module.exports = config;
