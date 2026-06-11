<script setup lang="ts">
const props = defineProps<{ value: number; max?: number }>();

const pct = computed(() => {
  const max = props.max ?? 100;
  if (!max) return 0;
  return Math.max(0, Math.min(100, (props.value / max) * 100));
});
</script>

<template>
  <div class="t-progress" :aria-valuenow="value" :aria-valuemax="max ?? 100" role="progressbar">
    <div class="t-progress__fill" :style="{ '--p': `${pct}%` }" />
  </div>
</template>
