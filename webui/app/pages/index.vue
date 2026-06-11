<script setup lang="ts">
import { ago, sumCount } from "~/utils/format";

const { data: snap } = await useFetch("/api/snapshot");

const dotState = (jobsActive: number) => jobsActive > 0 ? "active" : "idle";

const readyOk = computed(() => {
  const checks = snap.value?.health.readyz.checks ?? {};
  return Object.values(checks).every(v => v === "ok");
});
</script>

<template>
  <main v-if="snap" class="t-page t-page--wide">
    <header style="display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; padding-bottom: 28px; border-bottom: 1px solid var(--rule); margin-bottom: 40px; flex-wrap: wrap;">
      <div>
        <TEyebrow bare>Daemon</TEyebrow>
        <TTitle size="md" style="margin-top: 12px;">Pools</TTitle>
        <div class="t-meta t-meta--soft" style="margin-top: 12px;">
          <span>{{ snap.process.hostname }}</span><span class="t-pip">·</span>
          <span>started {{ ago(snap.process.started_at) }}</span>
        </div>
      </div>
      <div class="t-meta t-meta--soft" style="text-align: right;">
        <span style="display: inline-flex; align-items: center;">
          <TPipDot :state="snap.health.healthz.status === 'ok' ? 'active' : 'warn'" />
          /healthz
        </span>
        <span class="t-pip">·</span>
        <span style="display: inline-flex; align-items: center;">
          <TPipDot :state="readyOk ? 'active' : 'warn'" />
          /readyz
        </span>
      </div>
    </header>

    <table class="t-table">
      <thead>
        <tr>
          <th>Pool</th>
          <th>Executor</th>
          <th>Active</th>
          <th>Success</th>
          <th>Failure</th>
          <th>Errors</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in snap.pools" :key="p.name" @click="navigateTo(`/pools/${p.name}`)">
          <td>
            <span style="display: inline-flex; align-items: center;">
              <TPipDot :state="dotState(p.jobs_active)" />
              {{ p.name }}
            </span>
          </td>
          <td><span class="t-mono">{{ p.executor }}</span></td>
          <td>
            <div style="display: flex; align-items: center; gap: 10px;">
              <span class="t-mono">{{ p.jobs_active }} / {{ p.concurrency }}</span>
              <TProgress :value="p.jobs_active" :max="p.concurrency" style="width: 60px;" />
            </div>
          </td>
          <td><span class="t-mono">{{ (p.jobs_total.find(r => r.status === "success")?.count ?? 0).toLocaleString() }}</span></td>
          <td>
            <span class="t-mono" :style="(p.jobs_total.find(r => r.status === 'failure')?.count ?? 0) > 0 ? 'color: var(--warning);' : ''">
              {{ (p.jobs_total.find(r => r.status === "failure")?.count ?? 0).toLocaleString() }}
            </span>
          </td>
          <td>
            <span class="t-mono" :style="sumCount(p.job_errors_total) > 0 ? 'color: var(--warning);' : ''">
              {{ sumCount(p.job_errors_total).toLocaleString() }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </main>
</template>
