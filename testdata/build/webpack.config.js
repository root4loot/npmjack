const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const { BundleAnalyzerPlugin } = require('webpack-bundle-analyzer');

module.exports = {
  entry: './src/index.js',
  mode: 'production',
  externals: {
    'react': 'React',
    'react-dom': 'ReactDOM',
    'lodash': '_',
    'jquery': '$',
    'missing-external': 'MissingExternal',
    'unclaimed-lib': 'UnclaimedLib'
  },
  plugins: [
    new HtmlWebpackPlugin({
      template: './public/index.html'
    }),
    new MiniCssExtractPlugin({
      filename: '[name].[contenthash].css'
    }),
    new BundleAnalyzerPlugin({
      analyzerMode: 'static',
      reportFilename: 'bundle-report.html'
    }),
    // Custom plugin referencing missing package
    require('missing-webpack-plugin')(),
    require('vulnerable-plugin-123')({
      option: 'value'
    })
  ],
  module: {
    rules: [
      {
        test: /\.jsx?$/,
        exclude: /node_modules/,
        use: {
          loader: 'babel-loader',
          options: {
            presets: [
              '@babel/preset-env',
              '@babel/preset-react',
              'missing-babel-preset'
            ],
            plugins: [
              '@babel/plugin-transform-runtime',
              'babel-plugin-missing-transform'
            ]
          }
        }
      },
      {
        test: /\.css$/,
        use: [
          MiniCssExtractPlugin.loader,
          'css-loader',
          'postcss-loader'
        ]
      }
    ]
  },
  resolve: {
    alias: {
      '@components': path.resolve(__dirname, 'src/components'),
      'missing-alias': 'missing-alias-target',
      'utils': 'utility-package-unclaimed'
    }
  }
};