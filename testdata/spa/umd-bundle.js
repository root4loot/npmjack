// UMD bundle with various patterns
(function (global, factory) {
    typeof exports === 'object' && typeof module !== 'undefined' ? factory(require('lodash'), require('moment')) :
    typeof define === 'function' && define.amd ? define(['lodash', 'moment'], factory) :
    (global = global || self, factory(global.lodash, global.moment));
}(this, function (lodash, moment) {
    'use strict';

    // Global assignments
    window.React = React;
    global['vue'] = Vue;
    window["chart.js"] = ChartJS;
    
    // UMD factory patterns
    factory(require('axios'), require('express'));
    
    // Global variable patterns
    global.MissingUMDPackage = {};
    window.UnclaimedUMDLib = function() {};
}));