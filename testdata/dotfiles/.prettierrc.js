module.exports = {
  semi: true,
  trailingComma: 'all',
  singleQuote: true,
  printWidth: 100,
  tabWidth: 2,
  plugins: [
    'prettier-plugin-organize-imports',
    'prettier-plugin-tailwindcss',
    '@prettier/plugin-php',
    'prettier-plugin-missing',
    require('prettier-plugin-inline'),
    require.resolve('prettier-plugin-unclaimed'),
  ],
  overrides: [
    {
      files: '*.md',
      options: {
        parser: 'markdown',
        plugins: ['prettier-plugin-markdown-missing']
      }
    }
  ]
};