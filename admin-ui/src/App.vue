<script setup lang="ts">
import UiToastHost from './components/ui/UiToastHost.vue'
</script>

<template>
  <a class="skip-link" href="#main-content">跳到主要内容</a>
  <router-view />
  <UiToastHost />
</template>
