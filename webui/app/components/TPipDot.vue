<script setup lang="ts">
withDefaults(defineProps<{
  state?: "active" | "idle" | "warn" | "off";
}>(), {
  state: "idle",
});
</script>

<template>
  <span
    class="t-pip-dot"
    :class="{
      't-pip-dot--active': state === 'active',
      't-pip-dot--idle': state === 'idle',
      't-pip-dot--warn': state === 'warn',
      't-pip-dot--off': state === 'off',
    }"
  />
</template>
