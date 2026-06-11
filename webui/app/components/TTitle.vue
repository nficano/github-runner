<script setup lang="ts">
withDefaults(defineProps<{
  size?: "display" | "h1" | "md" | "sm" | "xs";
  as?: string;
}>(), {
  size: "h1",
  as: "h1",
});
</script>

<template>
  <component
    :is="as"
    class="t-title"
    :class="{
      't-title--display': size === 'display',
      't-title--md': size === 'md',
      't-title--sm': size === 'sm',
      't-title--xs': size === 'xs',
    }"
  >
    <slot />
  </component>
</template>
