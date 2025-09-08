// AMD/RequireJS patterns
define(['jquery', 'underscore', 'backbone', 'missing-amd-dep'], function($, _, Backbone, missing) {
  // Module definition
  return {
    init: function() {
      console.log('Initialized');
    }
  };
});

// RequireJS with callback
require(['moment', 'chart.js', 'unclaimed-chart-plugin'], function(moment, Chart, plugin) {
  // Usage code
});

// Nested require calls
define(function(require) {
  var $ = require('jquery');
  var plugin = require('jquery-missing-plugin');
  var utils = require('utility-functions');
  
  return function() {
    // Module logic
  };
});

// Mixed with paths config (common in real apps)
requirejs.config({
  paths: {
    'jquery': 'lib/jquery-3.6.0',
    'missing-lib': 'vendor/missing-lib',
    'potential-squat': 'js/potential-squat'
  }
});

require(['jquery', 'missing-lib'], function($, lib) {
  // App code
});