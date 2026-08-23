<script setup lang="ts">
import { ref } from 'vue'
import { IconPhotoPlus, IconTrash } from '@tabler/icons-vue'
import AssetThumbnail from './AssetThumbnail.vue'

withDefaults(defineProps<{
  path?: string
  label: string
  kind?: 'pet' | 'item' | 'shop' | 'image'
  size?: 'small' | 'medium' | 'large'
  busy?: boolean
}>(), {
  path: '',
  kind: 'image',
  size: 'large',
  busy: false,
})

const emit = defineEmits<{
  file: [file: File]
  clear: []
}>()

const dragging = ref(false)
const input = ref<HTMLInputElement | null>(null)

function selectFile(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) emit('file', file)
  target.value = ''
}

function dropFile(event: DragEvent) {
  dragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) emit('file', file)
}
</script>

<template>
  <section class="image-editor" aria-label="图片上传与预览">
    <AssetThumbnail :path="path" :label="label" :kind="kind" :size="size" />
    <div
      class="image-dropzone"
      :class="{ 'is-dragging': dragging, 'is-busy': busy }"
      tabindex="0"
      role="button"
      :aria-disabled="busy"
      @click="!busy && input?.click()"
      @keydown.enter.prevent="!busy && input?.click()"
      @keydown.space.prevent="!busy && input?.click()"
      @dragenter.prevent="dragging = true"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="dropFile"
    >
      <IconPhotoPlus :size="24" />
      <div><strong>{{ busy ? '图片上传中…' : '点击选择或拖拽图片到这里' }}</strong><small>JPG、PNG、GIF 或 WEBP，最大 10MB</small></div>
      <input ref="input" type="file" accept="image/png,image/jpeg,image/gif,image/webp" :disabled="busy" @change="selectFile" />
    </div>
    <button v-if="path" type="button" class="clear-image" :disabled="busy" @click.stop="emit('clear')"><IconTrash :size="14" />移除当前图片</button>
  </section>
</template>

<style scoped>
.image-editor{display:grid;gap:10px}.image-dropzone{display:grid;grid-template-columns:auto minmax(0,1fr);align-items:center;gap:11px;padding:14px;border:1px dashed var(--border-strong);border-radius:12px;background:var(--bg-elevated);color:var(--accent);cursor:pointer;transition:border-color .2s ease,background .2s ease,transform .2s var(--ease-out)}
.image-dropzone:hover,.image-dropzone:focus-visible,.image-dropzone.is-dragging{border-color:var(--accent);background:var(--accent-soft);outline:0}.image-dropzone.is-dragging{transform:translateY(-2px)}.image-dropzone.is-busy{cursor:wait;opacity:.72}.image-dropzone div{display:grid;gap:2px}.image-dropzone strong{color:var(--text-main);font-size:13px}.image-dropzone small{color:var(--text-muted);font-size:11px}.image-dropzone input{position:absolute;width:1px;height:1px;overflow:hidden;opacity:0;pointer-events:none}.clear-image{display:inline-flex;align-items:center;justify-self:start;gap:5px;padding:4px 0;border:0;background:transparent;color:var(--danger);font:inherit;font-size:11px;cursor:pointer}.clear-image:disabled{cursor:not-allowed;opacity:.55}
</style>
