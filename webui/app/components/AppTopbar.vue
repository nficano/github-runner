<script setup lang="ts">
const route = useRoute();

const nav = [
  { to: "/", label: "Pools", match: (p: string) => p === "/" || p.startsWith("/pools") },
  { to: "/activity", label: "Activity", match: (p: string) => p.startsWith("/activity") },
  { to: "/health", label: "Health", match: (p: string) => p.startsWith("/health") },
  { to: "/configuration", label: "Config", match: (p: string) => p.startsWith("/configuration") },
];
</script>

<template>
  <header class="t-topbar">
    <NuxtLink to="/" class="t-wordmark">github-runner</NuxtLink>

    <nav class="t-nav" aria-label="Primary">
      <NuxtLink
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        :class="{ 'is-active': item.match(route.path) }"
      >
        {{ item.label }}
      </NuxtLink>
    </nav>

    <ThemeSwitcher />
  </header>
</template>
