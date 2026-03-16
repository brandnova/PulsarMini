/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    '../templates/**/*.html',
    '../static/**/*.js',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ink:          '#1a1a18',
        'ink-soft':   '#4a4a45',
        'ink-muted':  '#8a8a82',
        paper:        '#faf9f6',
        'paper-alt':  '#f2f0eb',
        rule:         '#e0ddd6',
        accent:       '#c8402a',
        'accent-alt': '#2a6e5e',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
};