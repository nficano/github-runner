<script setup lang="ts">
withDefaults(defineProps<{
  parts?: string[];
  tone?: "default" | "soft" | "accent" | "warn";
}>(), {
  parts: () => [],
  tone: "default",
});
</script>

<template>
  <div
    class="t-meta"
    :class="{
      't-meta--soft': tone === 'soft',
      't-meta--accent': tone === 'accent',
      't-meta--warn': tone === 'warn',
    }"
  >
    <template v-if="parts.length">
      <template v-for="(p, i) in parts" :key="i">
        <span>{{ p }}</span>
        <span v-if="i < parts.length - 1" class="t-pip">·</span>
      </template>
    </template>
    <slot v-else />
  </div>
</template>
