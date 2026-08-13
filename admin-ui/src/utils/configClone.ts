import { toRaw } from 'vue'

export function cloneConfigValue<T>(value: T): T {
  return JSON.parse(JSON.stringify(toRaw(value))) as T
}
