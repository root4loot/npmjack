const path = require('path');

module.exports = {
  entry: './src/index.js',
  externals: {
    'react': 'React',
    'react-dom': 'ReactDOM', 
    'lodash': '_',
    '@babel/core': 'BabelCore',
    'missing-external-lib': 'MissingLib',
    'vulnerable-external': 'VulnerableExt'
  },
  resolve: {
    alias: {
      'utils': 'missing-alias-package'
    }
  },
  plugins: [
    new webpack.DefinePlugin({
      'process.env.NODE_ENV': JSON.stringify('production')
    })
  ]
};