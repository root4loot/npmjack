import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';

export default defineConfig({
  plugins: [
    react(),
    // Missing/unclaimed plugins
    require('vite-plugin-missing')(),
    require('unclaimed-vite-plugin')({
      option: true
    })
  ],
  build: {
    rollupOptions: {
      external: [
        'react',
        'react-dom',
        'missing-external-dep',
        'unclaimed-rollup-external'
      ],
      output: {
        globals: {
          'react': 'React',
          'react-dom': 'ReactDOM',
          'missing-external-dep': 'MissingDep',
          'unclaimed-rollup-external': 'UnclaimedExternal'
        }
      }
    }
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      'components': resolve(__dirname, 'src/components'),
      'missing-alias': 'missing-package-target'
    }
  },
  optimizeDeps: {
    include: [
      'lodash',
      'moment',
      'missing-optimization-dep'
    ],
    exclude: [
      'large-package',
      'excluded-missing-package'
    ]
  },
  define: {
    __PACKAGE_VERSION__: JSON.stringify(require('missing-version-package').version)
  }
});