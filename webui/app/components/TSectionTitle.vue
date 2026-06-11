<script setup lang="ts">
withDefaults(defineProps<{
  align?: "center" | "left";
}>(), { align: "center" });
</script>

<template>
  <h2
    class="t-section-title"
    :class="{ 't-section-title--left': align === 'left' }"
  >
    <slot />
  </h2>
</template>
