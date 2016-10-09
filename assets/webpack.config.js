var webpack = require('webpack');
var path = require('path');

var ExtractTextPlugin = require("extract-text-webpack-plugin");
var extractHTML = new ExtractTextPlugin('index.html');
var extractCSS = new ExtractTextPlugin('main.css');

var dist = path.resolve(__dirname, 'dist');
var src = path.resolve(__dirname, 'src');

var config = [{
  entry: path.resolve(src, 'main.jsx'),
  output: {
    path: dist,
    filename: 'main.js'
  },
  module : {
    loaders : [
      { test : /\.jsx?/, loader : 'babel' },
      { test: /\.html$/, loader: extractHTML.extract('raw!html-minify') },
      { test: /\.css$/, loader: extractCSS.extract("css-loader?minimize") }
    ]
  },
  plugins: [
    extractHTML,
    extractCSS,
  ],
}];

module.exports = config;
