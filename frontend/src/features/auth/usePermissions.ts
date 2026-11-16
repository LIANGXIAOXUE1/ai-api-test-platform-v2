import { computed, inject } from 'vue'
import { platformAuthKey } from './auth'
export function usePermissions() {
  const auth = inject(platformAuthKey)
  const role = computed(() => auth?.user.value?.role || 'administrator')
  return { auth, isAdmin: computed(() => role.value === 'administrator'), readOnly: computed(() => role.value === 'viewer'), canExecute: computed(() => role.value !== 'viewer'), canAuto: computed(() => role.value === 'administrator' || (role.value === 'tester' && !!auth?.user.value?.allowAuto)) }
}
