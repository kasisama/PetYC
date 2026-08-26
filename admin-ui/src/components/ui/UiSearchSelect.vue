<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { IconChevronDown, IconSearch, IconX } from '@tabler/icons-vue'

export interface SearchSelectOption {
  value: string
  label: string
  group?: string
}

const props = withDefaults(defineProps<{
  modelValue: string
  options: SearchSelectOption[]
  placeholder?: string
  disabled?: boolean
}>(), { placeholder: '请选择', disabled: false })

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const open = ref(false)
const query = ref('')
const input = ref<HTMLInputElement | null>(null)

const selected = computed(() => props.options.find(option => option.value === props.modelValue))
const visibleOptions = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  if (!needle) return props.options
  return props.options.filter(option => `${option.label} ${option.group ?? ''}`.toLocaleLowerCase().includes(needle))
})

async function show() {
  if (props.disabled) return
  query.value = ''
  open.value = true
  await nextTick()
  input.value?.focus()
}

function choose(value: string) {
  emit('update:modelValue', value)
  open.value = false
  query.value = ''
}

function closeOnFocusOut(event: FocusEvent) {
  const container = event.currentTarget as HTMLElement
  if (event.relatedTarget instanceof Node && container.contains(event.relatedTarget)) return
  open.value = false
  query.value = ''
}
</script>

<template>
  <div class="search-select" :class="{ open, disabled }" @focusout="closeOnFocusOut">
    <button class="search-select-trigger" type="button" :disabled="disabled" @click="open ? open=false : show()">
      <span :class="{ placeholder: !selected }">{{ selected?.label ?? placeholder }}</span>
      <IconChevronDown :size="16" />
    </button>
    <div v-if="open" class="search-select-popover">
      <label><IconSearch :size="15"/><input ref="input" v-model="query" type="search" placeholder="输入名称搜索" @keydown.esc.prevent="open=false"/></label>
      <div class="search-select-options" role="listbox">
        <button v-if="modelValue" type="button" class="clear" @click="choose('')"><IconX :size="14"/>清除选择</button>
        <button v-for="option in visibleOptions" :key="option.value" type="button" :class="{ selected: option.value===modelValue }" @click="choose(option.value)">
          <span>{{ option.label }}</span><small v-if="option.group">{{ option.group }}</small>
        </button>
        <p v-if="visibleOptions.length===0">没有匹配项</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-select{position:relative;min-width:0}.search-select-trigger{display:flex;align-items:center;justify-content:space-between;gap:10px;width:100%;min-height:41px;padding:9px 11px;border:1px solid var(--border-color);border-radius:9px;background:var(--bg-base);color:var(--text-main);font:inherit;text-align:left;cursor:pointer}.search-select.open .search-select-trigger{border-color:var(--accent)}.search-select-trigger .placeholder{color:var(--text-muted)}.search-select-trigger:disabled{cursor:not-allowed;opacity:.58}.search-select-popover{position:absolute;z-index:30;top:calc(100% + 5px);left:0;width:100%;min-width:240px;padding:7px;border:1px solid var(--border-strong);border-radius:10px;background:var(--bg-elevated);box-shadow:0 16px 40px rgba(0,0,0,.24)}.search-select-popover>label{display:flex;align-items:center;gap:7px;padding:0 9px;border:1px solid var(--border-color);border-radius:8px;color:var(--text-muted)}.search-select-popover input{width:100%;height:34px!important;padding:0!important;border:0!important;background:transparent!important;color:var(--text-main);outline:0}.search-select-options{display:grid;gap:2px;max-height:240px;margin-top:6px;overflow:auto}.search-select-options button{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:8px 9px;border:0;border-radius:7px;background:transparent;color:var(--text-main);font:inherit;font-size:12px;text-align:left;cursor:pointer}.search-select-options button:hover,.search-select-options button.selected{background:var(--accent-soft);color:var(--accent)}.search-select-options button.clear{justify-content:flex-start;color:var(--text-muted)}.search-select-options small{color:var(--text-muted);font-size:10px}.search-select-options p{margin:0;padding:14px;color:var(--text-muted);font-size:11px;text-align:center}
</style>
