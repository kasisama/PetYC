<script setup lang="ts">
defineProps<{ title: string; description?: string; eyebrow?: string }>()
</script>

<template>
  <header class="page-header">
    <div>
      <span v-if="eyebrow" class="page-eyebrow">{{ eyebrow }}</span>
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="page-header-actions"><slot name="actions" /></div>
  </header>
</template>

<style scoped>
.page-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; margin-bottom: 22px; }
.page-eyebrow { display: block; margin-bottom: 6px; color: var(--accent); font-size: 11px; font-weight: 760; letter-spacing: .12em; text-transform: uppercase; }
h1 { margin: 0; font-size: clamp(24px, 2.4vw, 34px); line-height: 1.15; letter-spacing: -.035em; }
p { max-width: 720px; margin: 8px 0 0; color: var(--text-muted); }
.page-header-actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; flex-wrap: wrap; }
@media (max-width: 700px) { .page-header { align-items: stretch; flex-direction: column; gap: 14px; } .page-header-actions { justify-content: flex-start; } }
</style>
