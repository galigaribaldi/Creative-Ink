import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { initTheme } from './lib/theme.svelte.js'

// Before mount(): <html data-theme> is set before the first frame, so the app
// never paints with the wrong theme.
initTheme()

const app = mount(App, {
  target: document.getElementById('app')
})

export default app
