<script setup lang="ts">
const { theme, apply } = useTheme();
</script>

<template>
  <div class="t-switcher" role="tablist" aria-label="Theme">
    <button
      type="button"
      :class="{ 'is-active': theme === 'day' }"
      @click="apply('day')"
    >Day</button>
    <button
      type="button"
      :class="{ 'is-active': theme === 'night' }"
      @click="apply('night')"
    >Night</button>
  </div>
</template>
