<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconBox, IconPaw, IconPhoto, IconShoppingBag } from '@tabler/icons-vue'
import { imagePreviewUrl } from '../../api/config'

const props = withDefaults(defineProps<{
  path?: string
  label: string
  kind?: 'pet' | 'item' | 'shop' | 'image'
  size?: 'small' | 'medium' | 'large' | 'catalog' | 'tile'
}>(), {
  path: '',
  kind: 'image',
  size: 'medium',
})

const failed = ref(false)
const source = computed(() => imagePreviewUrl(props.path))
const fallbackLabel = computed(() => [...props.label.trim()][0] || '图')

watch(() => props.path, () => {
  failed.value = false
})
</script>

<template>
  <div class="asset-thumbnail" :class="[`is-${size}`, `is-${kind}`, { 'is-empty': !source || failed }]">
    <img v-if="source && !failed" :src="source" :alt="`${label}图片`" @error="failed = true" />
    <div v-else class="thumbnail-fallback" role="img" :aria-label="`${label}暂无可用图片`">
      <IconPaw v-if="kind === 'pet'" :size="size === 'large' || size === 'catalog' || size === 'tile' ? 34 : 21" />
      <IconShoppingBag v-else-if="kind === 'shop'" :size="size === 'large' || size === 'catalog' || size === 'tile' ? 34 : 21" />
      <IconBox v-else-if="kind === 'item'" :size="size === 'large' || size === 'catalog' || size === 'tile' ? 34 : 21" />
      <IconPhoto v-else :size="size === 'large' || size === 'catalog' || size === 'tile' ? 34 : 21" />
      <span>{{ fallbackLabel }}</span>
    </div>
  </div>
</template>

<style scoped>
.asset-thumbnail{position:relative;display:grid;flex:0 0 auto;place-items:center;overflow:hidden;border:1px solid color-mix(in srgb,var(--border-color) 82%,transparent);border-radius:12px;background:var(--bg-elevated);color:var(--text-muted)}
.asset-thumbnail.is-small{width:40px;height:40px;border-radius:9px}.asset-thumbnail.is-medium{width:62px;height:62px}.asset-thumbnail.is-large{width:100%;height:260px;border-radius:14px}
.asset-thumbnail.is-catalog{width:100%;height:168px;border:0;border-radius:0;background:color-mix(in srgb,var(--bg-base) 88%,var(--accent-soft))}
.asset-thumbnail.is-tile{width:100%;height:100%;border:0;border-radius:0}
.asset-thumbnail img{display:block;width:100%;height:100%;object-fit:cover;object-position:center}.asset-thumbnail.is-large img,.asset-thumbnail.is-catalog img,.asset-thumbnail.is-tile img{width:auto;height:auto;max-width:calc(100% - 20px);max-height:calc(100% - 20px);object-fit:contain;object-position:center;background:transparent}
.thumbnail-fallback{position:relative;display:grid;width:100%;height:100%;place-items:center;background:radial-gradient(circle at 24% 20%,color-mix(in srgb,var(--accent) 18%,transparent),transparent 58%),var(--bg-elevated);color:var(--accent)}
.thumbnail-fallback span{position:absolute;right:6px;bottom:3px;color:color-mix(in srgb,var(--accent) 58%,var(--text-muted));font-size:10px;font-weight:700}.is-large .thumbnail-fallback span{right:12px;bottom:9px;font-size:13px}
</style>
