<script setup lang="ts">
withDefaults(defineProps<{
  label: string;
  value: string | number;
  hint?: string;
  size?: "default" | "sm";
}>(), {
  size: "default",
});
</script>

<template>
  <div class="t-stat">
    <div class="t-stat__label">{{ label }}</div>
    <div class="t-stat__value" :class="{ 't-stat__value--sm': size === 'sm' }">
      {{ value }}
    </div>
    <div v-if="hint" class="t-mono" style="opacity: 0.7;">{{ hint }}</div>
  </div>
</template>
