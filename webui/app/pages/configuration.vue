<script setup lang="ts">
import { ago } from "~/utils/format";

const { data: cfg } = await useFetch("/api/configuration");
const { data: snap } = await useFetch("/api/snapshot");

const numberedLines = computed(() =>
  (cfg.value?.toml ?? "").split("\n").map((line, i) => ({ n: i + 1, line }))
);
</script>

<template>
  <main v-if="cfg && snap" class="t-page t-page--wide">
    <header style="padding-bottom: 28px; border-bottom: 1px solid var(--rule); margin-bottom: 40px;">
      <TEyebrow bare>{{ cfg.path }}</TEyebrow>
      <TTitle size="md" style="margin-top: 12px;">Configuration</TTitle>
      <div class="t-meta t-meta--soft" style="margin-top: 12px;">
        <span>last reload {{ ago(cfg.last_reloaded) }}</span>
        <span class="t-pip">·</span>
        <span>reload via <code>{{ cfg.reload_signal }}</code></span>
      </div>
    </header>

    <!-- Resolved global config (post env interpolation, post defaults) -->
    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">Global · resolved</h2>

    <dl class="t-dl" style="margin-bottom: 56px;">
      <dt>log_level</dt><dd><code>{{ snap.global.log_level }}</code></dd>
      <dt>log_format</dt><dd><code>{{ snap.global.log_format }}</code></dd>
      <dt>metrics_listen</dt><dd><code>{{ snap.global.metrics_listen }}</code></dd>
      <dt>health_listen</dt><dd><code>{{ snap.global.health_listen }}</code></dd>
      <dt>check_interval</dt><dd><code>{{ snap.global.check_interval }}</code></dd>
      <dt>shutdown_timeout</dt><dd><code>{{ snap.global.shutdown_timeout }}</code></dd>
      <dt>api.base_url</dt><dd><code>{{ snap.global.api.base_url }}</code></dd>
      <dt>api.timeout</dt><dd><code>{{ snap.global.api.timeout }}</code></dd>
      <dt>api.max_retries</dt><dd><code>{{ snap.global.api.max_retries }}</code></dd>
    </dl>

    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">TOML</h2>

    <div
      class="theater-night"
      style="border-radius: var(--t-radius); padding: 20px 24px; background: var(--bg-deep); box-shadow: var(--t-shadow-stage);"
    >
      <pre style="font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.7; color: var(--ink-soft); margin: 0; overflow-x: auto;"><span v-for="row in numberedLines" :key="row.n"><span style="color: var(--muted); display: inline-block; width: 36px; text-align: right; padding-right: 16px; border-right: 1px solid var(--rule); user-select: none; margin-right: 16px;">{{ row.n }}</span>{{ row.line }}
</span></pre>
    </div>
  </main>
</template>
