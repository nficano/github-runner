<script setup lang="ts">
defineProps<{
  flex?: boolean;
  bare?: boolean;
}>();
</script>

<template>
  <div
    class="t-eyebrow"
    :class="{ 't-eyebrow--flex': flex, 't-eyebrow--bare': bare }"
  >
    <slot />
  </div>
</template>
