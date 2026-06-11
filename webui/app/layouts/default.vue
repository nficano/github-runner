<script setup lang="ts">
const { theme } = useTheme();
const themeClass = computed(() => `theater-${theme.value}`);
</script>

<template>
  <div :class="themeClass">
    <AppTopbar />
    <slot />
    <AppFooter />
  </div>
</template>
