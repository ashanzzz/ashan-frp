/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        dark: {
          900: '#070a0f',
          800: '#0d121a',
          700: '#141c28',
          600: '#1d2738',
        },
        brand: {
          blue: '#5b8cff',
          teal: '#30c98f',
          amber: '#f2c94c',
          red: '#eb5757',
        }
      }
    },
  },
  plugins: [],
}
