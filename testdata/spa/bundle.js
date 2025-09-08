// Bundle with CDN references and comments
/* webpack bundle for react-app */
const React = require("react");
const lodash = require("lodash");

// Webpack chunk comment
/*** WEBPACK CHUNK: @babel/core ***/
function __webpack_require__(123) { }

// CDN references in comments
/* Using https://unpkg.com/axios@latest */
/* Loading from https://cdn.jsdelivr.net/npm/chart.js */

// Script tags in content
const scriptContent = `
  <script src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.29.1/moment.min.js"></script>
`;

// Import map example
const importMap = {
  "vue": "https://unpkg.com/vue@3/dist/vue.esm-browser.js",
  "rxjs": "https://cdn.jsdelivr.net/npm/rxjs@7.5.0/dist/bundles/rxjs.umd.min.js"
};