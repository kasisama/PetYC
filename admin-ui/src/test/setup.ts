import { afterEach } from 'vitest'

afterEach(() => {
  localStorage.clear()
  document.documentElement.removeAttribute('data-density')
})
