import './base.css'
import './pages.css'
import { call } from './api'

const install = document.querySelector<HTMLElement>('#install')!
const code = document.querySelector<HTMLInputElement>('#code')!
const err = document.querySelector<HTMLElement>('#err')!

fetch('/__demo/state').then(async (r) => {
  if (r.ok) location.replace('/') // already unlocked
}).catch(() => {})
// The install ID is public (the PIN app needs it): it rides on the 401's page too.
call<{ installId: string }>('/__demo/install').then((s) => (install.textContent = s.installId)).catch(() => (install.textContent = 'this demo'))

code.addEventListener('input', () => (code.value = code.value.replace(/\D/g, '')))
document.querySelector('form')!.addEventListener('submit', async (e) => {
  e.preventDefault()
  err.textContent = ''
  try {
    await call('/__demo/unlock', { code: code.value })
    location.replace('/')
  } catch (x) {
    err.textContent = (x as Error).message
    code.select()
  }
})
