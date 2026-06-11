<script setup lang="ts">
withDefaults(defineProps<{
  to?: string;
  flat?: boolean;
}>(), {});
</script>

<template>
  <NuxtLink v-if="to" :to="to" class="t-card-link">
    <slot />
  </NuxtLink>
  <div v-else class="t-card" :class="{ 't-card--flat': flat }">
    <slot />
  </div>
</template>
