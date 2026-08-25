<script setup lang="ts">
import { computed } from 'vue'
import { IconChevronLeft, IconChevronRight } from '@tabler/icons-vue'

const props = withDefaults(defineProps<{ page:number; limit:number; total:number }>(), { page:1, limit:20, total:0 })
const emit = defineEmits<{ change:[page:number]; 'update:limit':[limit:number] }>()
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.limit)))
const from = computed(() => props.total === 0 ? 0 : (props.page - 1) * props.limit + 1)
const to = computed(() => Math.min(props.total, props.page * props.limit))
</script>

<template>
  <nav class="pagination" aria-label="列表分页">
    <span>第 {{ from }}–{{ to }} 条，共 {{ total }} 条</span>
    <label>每页<select :value="limit" @change="emit('update:limit', Number(($event.target as HTMLSelectElement).value))"><option :value="20">20</option><option :value="50">50</option><option :value="100">100</option></select></label>
    <button class="ui-icon-button" :disabled="page <= 1" aria-label="上一页" @click="emit('change', page - 1)"><IconChevronLeft :size="17" /></button>
    <b>{{ page }} / {{ pages }}</b>
    <button class="ui-icon-button" :disabled="page >= pages" aria-label="下一页" @click="emit('change', page + 1)"><IconChevronRight :size="17" /></button>
  </nav>
</template>

<style scoped>
.pagination{display:flex;align-items:center;justify-content:flex-end;gap:9px;margin-top:10px;padding:10px 12px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-surface)}.pagination>span{margin-right:auto;color:var(--text-muted);font-size:12px}.pagination label{display:flex;align-items:center;gap:6px;color:var(--text-muted);font-size:12px}.pagination select{min-height:34px;padding:4px 8px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-base);color:var(--text-main)}.pagination b{min-width:54px;text-align:center;font-size:12px}@media(max-width:560px){.pagination{flex-wrap:wrap;justify-content:center}.pagination>span{width:100%;margin:0;text-align:center}.pagination label{margin-right:auto}}
</style>
