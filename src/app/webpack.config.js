'use strict;';

const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const webpack = require('webpack');

const isProd = process.env.NODE_ENV === 'production';

const config = {
    entry: './src/app/index.tsx',
    output: {
        filename: '[name].[chunkhash].js',
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
                loaders: [ ...( isProd ? [] : ['react-hot-loader/webpack']), 'awesome-typescript-loader?configFileName=./src/app/tsconfig.json']
            }, {
                enforce: 'pre',
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
            SYSTEM_INFO: JSON.stringify({
                version: process.env.ARGO_VERSION || 'latest',
            }),
        }),
        new HtmlWebpackPlugin({ template: 'src/app/index.html' }),
        new CopyWebpackPlugin([{
            from: 'node_modules/argo-ui/bundle/assets', to: 'assets'
        }, {
            from: 'node_modules/font-awesome/fonts', to: 'assets/fonts'
        }]),
    ],
    devServer: {
        historyApiFallback: true,
        port: 4000,
        proxy: {
            '/api': {
                'target': process.env.ARGO_API_URL || 'http://localhost:8080',
                'secure': false,
            }
        }
    }
};

if (isProd) {
    config
        .plugins
        .push(new webpack.optimize.UglifyJsPlugin());
}

module.exports = config;
