<script setup lang="ts">
import { uptime } from "~/utils/format";

const { data: snap } = await useFetch("/api/snapshot");
</script>

<template>
  <footer class="t-footer">
    <span>{{ snap?.process.hostname }}</span><span class="t-pip">·</span>
    <span>github-runner {{ snap?.process.build_version }}</span>
    <template v-if="snap?.process.runtime_collector_registered">
      <span class="t-pip">·</span>
      <span>uptime {{ uptime(snap.process.runtime.uptime_seconds) }}</span>
    </template>
  </footer>
</template>
