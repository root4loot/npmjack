// Minified bundle patterns
!function(e,t){"object"==typeof exports&&"object"==typeof module?module.exports=t(require("react"),require("lodash")):"function"==typeof define&&define.amd?define(["react","lodash"],t):"object"==typeof exports?exports.App=t(require("react"),require("lodash")):e.App=t(e.React,e._)}(window,(function(e,t){
var n=function(e){return parcel$require("moment")};
var r=function(){return e("axios")};
var i=function(){return require("missing-minified-pkg")};

// Webpack module patterns
/*** webpack bundle ***/ 
function __webpack_require__(moduleId) {
  // webpack module loading
}
// Module 123: react
// Module 456: vue
__webpack_require__(123);

// Rollup bundle marker
// rollup bundle version 1.2.3 require('chart.js')
// rollup: require('vulnerable-rollup-pkg')

// Parcel patterns  
parcel$require("unclaimed-parcel-lib");

// Common minified calls
n("missing-bundle-dep");
r("bundle-helper");
}));