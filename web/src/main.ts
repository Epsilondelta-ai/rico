import { mount } from 'svelte'
import './app.css'
import './lib/i18n' // i18n 초기화
import App from './App.svelte'

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
