<script setup lang="ts">
withDefaults(defineProps<{
  tone?: "default" | "accent" | "warn";
}>(), {
  tone: "default",
});
</script>

<template>
  <span
    class="t-chip"
    :class="{ 't-chip--accent': tone === 'accent', 't-chip--warn': tone === 'warn' }"
  >
    <slot />
  </span>
</template>
