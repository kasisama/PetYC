<script setup lang="ts">
import { IconArrowLeft, IconHome, IconMapQuestion } from '@tabler/icons-vue'
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <section class="not-found">
    <IconMapQuestion :size="54" />
    <span>404 · 页面未找到</span>
    <h1>这里没有对应的运营页面</h1>
    <p>地址可能已变更，现有数据不会受到影响。你可以返回上一页，或回到运营总览。</p>
    <div><button class="btn btn-ghost" @click="router.back()"><IconArrowLeft :size="17" />返回上一页</button><RouterLink class="btn btn-primary" :to="{ name: 'dashboard' }"><IconHome :size="17" />运营总览</RouterLink></div>
  </section>
</template>

<style scoped>
.not-found{display:grid;place-items:center;align-content:center;min-height:60dvh;padding:32px;text-align:center;color:var(--text-muted)}.not-found>svg{margin-bottom:18px;color:var(--accent)}.not-found>span{font-size:11px;font-weight:750;letter-spacing:.12em;text-transform:uppercase}.not-found h1{margin:8px 0;font-size:clamp(28px,5vw,48px);color:var(--text-main);letter-spacing:-.04em}.not-found p{max-width:560px;margin:0 0 24px;line-height:1.7}.not-found div{display:flex;gap:10px;flex-wrap:wrap;justify-content:center}.not-found .btn{display:inline-flex;align-items:center;gap:7px;text-decoration:none}
</style>
