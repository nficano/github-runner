<script setup lang="ts">
withDefaults(defineProps<{
  variant?: "primary" | "secondary" | "small" | "warn";
  to?: string;
  href?: string;
  type?: "button" | "submit" | "reset";
  disabled?: boolean;
}>(), {
  variant: "primary",
  type: "button",
});

const variantClass = (v: string) =>
  v === "secondary" ? "t-btn-secondary"
    : v === "small" ? "t-btn-sm"
    : v === "warn" ? "t-btn t-btn-warn"
    : "t-btn";
</script>

<template>
  <NuxtLink v-if="to" :to="to" :class="variantClass(variant)">
    <slot />
  </NuxtLink>
  <a v-else-if="href" :href="href" :class="variantClass(variant)">
    <slot />
  </a>
  <button
    v-else
    :type="type"
    :disabled="disabled"
    :class="variantClass(variant)"
  >
    <slot />
  </button>
</template>
