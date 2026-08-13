<script setup lang="ts">
import { nextTick, onBeforeUnmount, watch } from 'vue'
import { IconX } from '@tabler/icons-vue'

const props=withDefaults(defineProps<{open:boolean;title:string;description?:string;busy?:boolean}>(),{description:'',busy:false})
const emit=defineEmits<{close:[]}>()
let previousFocus:HTMLElement|null=null
function close(){if(!props.busy)emit('close')}
function keydown(event:KeyboardEvent){if(event.key==='Escape')close()}
watch(()=>props.open,async open=>{if(open){previousFocus=document.activeElement as HTMLElement;document.body.style.overflow='hidden';window.addEventListener('keydown',keydown);await nextTick();document.querySelector<HTMLElement>('.ui-drawer-panel button, .ui-drawer-panel input, .ui-drawer-panel select, .ui-drawer-panel textarea')?.focus()}else{document.body.style.overflow='';window.removeEventListener('keydown',keydown);previousFocus?.focus()}})
onBeforeUnmount(()=>{document.body.style.overflow='';window.removeEventListener('keydown',keydown)})
</script>
<template>
  <Teleport to="body"><Transition name="drawer"><div v-if="open" class="ui-drawer" role="presentation" @mousedown.self="close">
    <aside class="ui-drawer-panel" role="dialog" aria-modal="true" :aria-label="title">
      <header><div><h2>{{title}}</h2><p v-if="description">{{description}}</p></div><button class="ui-icon-button" aria-label="关闭" @click="close"><IconX :size="20"/></button></header>
      <div class="ui-drawer-body"><slot/></div><footer v-if="$slots.footer"><slot name="footer"/></footer>
    </aside>
  </div></Transition></Teleport>
</template>
<style scoped>
.ui-drawer{position:fixed;inset:0;z-index:var(--z-modal);display:flex;justify-content:flex-end;background:rgba(18,10,15,.48);backdrop-filter:blur(3px)}.ui-drawer-panel{display:grid;grid-template-rows:auto 1fr auto;width:min(620px,94vw);height:100%;background:var(--bg-surface);border-left:1px solid var(--border-color);box-shadow:var(--shadow-dialog)}header{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;padding:22px 24px;border-bottom:1px solid var(--border-color)}h2{margin:0;font-size:21px}p{margin:5px 0 0;color:var(--text-muted)}.ui-drawer-body{overflow:auto;padding:24px}footer{padding:14px 24px;border-top:1px solid var(--border-color);background:var(--bg-elevated)}.drawer-enter-active,.drawer-leave-active{transition:opacity .18s ease}.drawer-enter-active .ui-drawer-panel,.drawer-leave-active .ui-drawer-panel{transition:transform .22s var(--ease-out)}.drawer-enter-from,.drawer-leave-to{opacity:0}.drawer-enter-from .ui-drawer-panel,.drawer-leave-to .ui-drawer-panel{transform:translateX(100%)}@media(max-width:600px){.ui-drawer{align-items:flex-end}.ui-drawer-panel{width:100%;height:min(88vh,760px);border-left:0;border-top:1px solid var(--border-color);border-radius:20px 20px 0 0}.drawer-enter-from .ui-drawer-panel,.drawer-leave-to .ui-drawer-panel{transform:translateY(100%)}header,.ui-drawer-body{padding:18px}}
</style>
