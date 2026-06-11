<script setup lang="ts">
import { ago, sumCount } from "~/utils/format";

const { data: snap } = await useFetch("/api/snapshot");

const byStatus = computed(() => {
  const m = new Map<string, number>();
  for (const p of snap.value?.pools ?? []) {
    for (const r of p.jobs_total) m.set(r.status, (m.get(r.status) ?? 0) + r.count);
  }
  return Array.from(m, ([status, count]) => ({ status, count }))
    .sort((a, b) => b.count - a.count);
});

const byRepo = computed(() => {
  const m = new Map<string, { success: number; failure: number; total: number }>();
  for (const p of snap.value?.pools ?? []) {
    for (const r of p.jobs_by_repository) {
      const cur = m.get(r.repository) ?? { success: 0, failure: 0, total: 0 };
      if (r.status === "success") cur.success += r.count;
      if (r.status === "failure") cur.failure += r.count;
      cur.total += r.count;
      m.set(r.repository, cur);
    }
  }
  return Array.from(m, ([repository, c]) => ({ repository, ...c }))
    .sort((a, b) => b.total - a.total);
});

const errorByType = computed(() => {
  const m = new Map<string, number>();
  for (const p of snap.value?.pools ?? []) {
    for (const e of p.job_errors_total) m.set(e.error_type, (m.get(e.error_type) ?? 0) + e.count);
  }
  return Array.from(m, ([error_type, count]) => ({ error_type, count }))
    .sort((a, b) => b.count - a.count);
});

const grandTotal = computed(() => byStatus.value.reduce((s, r) => s + r.count, 0));
</script>

<template>
  <main v-if="snap" class="t-page t-page--wide">
    <header style="padding-bottom: 28px; border-bottom: 1px solid var(--rule); margin-bottom: 40px;">
      <TEyebrow bare>Cumulative since {{ ago(snap.process.started_at) }}</TEyebrow>
      <TTitle size="md" style="margin-top: 12px;">Activity</TTitle>
    </header>

    <section style="display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); border: 1px solid var(--rule); border-radius: var(--t-radius); margin-bottom: 40px;">
      <div style="padding: 24px; border-right: 1px solid var(--rule);">
        <TStat label="Jobs total" :value="grandTotal.toLocaleString()" />
      </div>
      <div
        v-for="row in byStatus"
        :key="row.status"
        style="padding: 24px; border-right: 1px solid var(--rule);"
      >
        <TStat :label="row.status" :value="row.count.toLocaleString()" />
      </div>
    </section>

    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">By repository</h2>

    <table class="t-table" style="margin-bottom: 56px;">
      <thead>
        <tr>
          <th>Repository</th>
          <th>Total</th>
          <th>Success</th>
          <th>Failure</th>
          <th>Failure rate</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in byRepo" :key="r.repository">
          <td><span class="t-mono">{{ r.repository }}</span></td>
          <td><span class="t-mono">{{ r.total.toLocaleString() }}</span></td>
          <td><span class="t-mono">{{ r.success.toLocaleString() }}</span></td>
          <td>
            <span class="t-mono" :style="r.failure > 0 ? 'color: var(--warning);' : ''">
              {{ r.failure.toLocaleString() }}
            </span>
          </td>
          <td>
            <span class="t-mono">
              {{ r.total ? `${(100 * r.failure / r.total).toFixed(2)}%` : "—" }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>

    <h2 v-if="errorByType.length" class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">Errors by type</h2>

    <table v-if="errorByType.length" class="t-table">
      <thead>
        <tr><th>error_type</th><th>Count</th></tr>
      </thead>
      <tbody>
        <tr v-for="row in errorByType" :key="row.error_type">
          <td><span class="t-mono">{{ row.error_type }}</span></td>
          <td><span class="t-mono">{{ row.count.toLocaleString() }}</span></td>
        </tr>
      </tbody>
    </table>
  </main>
</template>
