<script setup lang="ts">
defineProps<{ error: { statusCode: number; message: string } }>();
const { theme } = useTheme();
const themeClass = computed(() => `theater-${theme.value}`);
</script>

<template>
  <div :class="themeClass">
    <main class="t-page" style="text-align: center; min-height: 80vh; display: flex; flex-direction: column; justify-content: center;">
      <TEyebrow>Curtain Down</TEyebrow>
      <TTitle size="display" style="margin-top: 24px;">{{ error.statusCode }}</TTitle>
      <p class="t-sub t-sub--lg" style="margin-top: 16px;">{{ error.message || "an unexpected exit" }}</p>
      <div style="margin-top: 56px;">
        <TBtn @click="clearError({ redirect: '/' })">Return to the Stage</TBtn>
      </div>
    </main>
  </div>
</template>
