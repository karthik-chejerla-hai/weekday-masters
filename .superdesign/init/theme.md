# Theme

## Compact token summary

- Typeface: system UI stack (`system-ui`, Apple, Segoe UI, Roboto, sans-serif).
- Primary: cyan 50 `#ecfeff`, 100 `#cffafe`, 200 `#a5f3fc`, 500 `#06b6d4`, 600 `#0891b2`, 700 `#0e7490`, 800 `#155e75`, 950 `#083344`.
- Secondary: amber 50 `#fffbeb`, 100 `#fef3c7`, 400 `#fbbf24`, 500 `#f59e0b`, 600 `#d97706`, 700 `#b45309`.
- Neutrals: Tailwind slate; body slate-50, text slate-900, borders slate-200.
- Status: green for success, amber for warning/low, red for danger/negative, blue for info.
- Shape: buttons/inputs 0.5rem radius, cards 0.75rem radius, chips fully rounded.
- Elevation: cards use `shadow-sm`; borders carry most separation.
- Layout: content max-width 56rem (`max-w-4xl`), 1rem mobile gutters, 1.5rem vertical page padding.
- Breakpoints: Tailwind defaults; bottom navigation hidden from `md` upward.

## Raw `frontend/tailwind.config.js`

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#ecfeff', 100: '#cffafe', 200: '#a5f3fc', 300: '#67e8f9',
          400: '#22d3ee', 500: '#06b6d4', 600: '#0891b2', 700: '#0e7490',
          800: '#155e75', 900: '#164e63', 950: '#083344',
        },
        secondary: {
          50: '#fffbeb', 100: '#fef3c7', 200: '#fde68a', 300: '#fcd34d',
          400: '#fbbf24', 500: '#f59e0b', 600: '#d97706', 700: '#b45309',
          800: '#92400e', 900: '#78350f', 950: '#451a03',
        },
      },
    },
  },
  plugins: [],
}
```

## Raw `frontend/src/index.css`

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  html { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
  body { @apply bg-slate-50 text-slate-900 antialiased; }
}

@layer components {
  .btn { @apply inline-flex items-center justify-center px-4 py-2 rounded-lg font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed; }
  .btn-primary { @apply btn bg-primary-600 text-white hover:bg-primary-700 focus:ring-primary-500; }
  .btn-secondary { @apply btn bg-secondary-500 text-white hover:bg-secondary-600 focus:ring-secondary-400; }
  .btn-outline { @apply btn border-2 border-primary-600 text-primary-600 hover:bg-primary-50 focus:ring-primary-500; }
  .btn-ghost { @apply btn text-slate-600 hover:bg-slate-100 focus:ring-slate-400; }
  .btn-danger { @apply btn bg-red-600 text-white hover:bg-red-700 focus:ring-red-500; }
  .card { @apply bg-white rounded-xl shadow-sm border border-slate-200; }
  .input { @apply w-full px-4 py-2 rounded-lg border border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent transition-colors; }
  .label { @apply block text-sm font-medium text-slate-700 mb-1; }
}
```

