/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,js,ts}'],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eef6ff', 100: '#d9eaff', 200: '#bcd9ff', 300: '#8ec0ff',
          400: '#599cff', 500: '#2f78fa', 600: '#1d5be0', 700: '#1848b5',
          800: '#163e92', 900: '#0f2a66'
        }
      }
    }
  },
  plugins: []
};
