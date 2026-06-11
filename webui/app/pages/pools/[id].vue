<script setup lang="ts">
import { pct, bytes } from "~/utils/format";

const route = useRoute();
const id = computed(() => route.params.id as string);

const { data: p, error } = await useFetch(() => `/api/pools/${id.value}`);

const dotState = (jobsActive: number) => jobsActive > 0 ? "active" : "idle";
</script>

<template>
  <main v-if="error" class="t-page" style="text-align: center;">
    <TEyebrow bare>404</TEyebrow>
    <TTitle size="md" style="margin-top: 24px;">Pool not configured</TTitle>
    <p class="t-body" style="margin-top: 16px;">no [[runners]] entry named <code>{{ id }}</code></p>
    <div style="margin-top: 40px;"><TBtn to="/">Back</TBtn></div>
  </main>

  <main v-else-if="p" class="t-page t-page--wide">
    <NuxtLink to="/" class="t-link-mono">← Pools</NuxtLink>

    <header style="display: flex; align-items: flex-end; justify-content: space-between; padding: 28px 0; border-bottom: 1px solid var(--rule); margin-bottom: 40px; flex-wrap: wrap; gap: 16px;">
      <div>
        <TEyebrow bare>{{ p.executor }} · {{ p.url.replace(/^https?:\/\//, "") }}</TEyebrow>
        <TTitle size="md" style="margin-top: 12px;">{{ p.name }}</TTitle>
        <div class="t-meta t-meta--soft" style="margin-top: 12px;">
          <span style="display: inline-flex; align-items: center;">
            <TPipDot :state="dotState(p.jobs_active)" />
            {{ p.jobs_active }} of {{ p.concurrency }} active
          </span>
          <span class="t-pip">·</span>
          <span>{{ p.ephemeral ? "ephemeral" : "long-lived" }}</span>
        </div>
      </div>
    </header>

    <!-- Jobs by repository -->
    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">Jobs by repository</h2>

    <table class="t-table" style="margin-bottom: 56px;">
      <thead>
        <tr><th>Repository</th><th>Status</th><th>Count</th></tr>
      </thead>
      <tbody>
        <tr v-for="(row, i) in p.jobs_by_repository" :key="i">
          <td><span class="t-mono">{{ row.repository }}</span></td>
          <td>
            <TChip :tone="row.status === 'failure' ? 'warn' : 'default'">{{ row.status }}</TChip>
          </td>
          <td><span class="t-mono">{{ row.count.toLocaleString() }}</span></td>
        </tr>
      </tbody>
    </table>

    <!-- Errors -->
    <h2 v-if="p.job_errors_total.length" class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">Errors by type</h2>

    <table v-if="p.job_errors_total.length" class="t-table" style="margin-bottom: 56px;">
      <thead>
        <tr><th>error_type</th><th>Count</th></tr>
      </thead>
      <tbody>
        <tr v-for="row in p.job_errors_total" :key="row.error_type">
          <td><span class="t-mono">{{ row.error_type }}</span></td>
          <td><span class="t-mono">{{ row.count.toLocaleString() }}</span></td>
        </tr>
      </tbody>
    </table>

    <!-- Cache -->
    <template v-if="p.cache?.type">
      <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">Cache · {{ p.cache.type }}</h2>

      <dl class="t-dl" style="margin-bottom: 24px;">
        <template v-if="p.cache.path">
          <dt>path</dt><dd><code>{{ p.cache.path }}</code></dd>
        </template>
        <dt>max_size</dt><dd><code>{{ p.cache.max_size }}</code></dd>
        <template v-if="p.cache.s3">
          <dt>s3.bucket</dt><dd><code>{{ p.cache.s3.bucket }}</code></dd>
          <dt>s3.region</dt><dd><code>{{ p.cache.s3.region }}</code></dd>
          <dt>s3.prefix</dt><dd><code>{{ p.cache.s3.prefix }}</code></dd>
        </template>
        <template v-if="p.cache.gcs">
          <dt>gcs.bucket</dt><dd><code>{{ p.cache.gcs.bucket }}</code></dd>
          <dt>gcs.prefix</dt><dd><code>{{ p.cache.gcs.prefix }}</code></dd>
        </template>
      </dl>

      <div v-if="p.cache_stats?.supports_stats" style="display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); border: 1px solid var(--rule); border-radius: var(--t-radius); margin-bottom: 56px;">
        <div style="padding: 18px; border-right: 1px solid var(--rule);">
          <TStat label="Entries" :value="p.cache_stats.entries.toLocaleString()" size="sm" />
        </div>
        <div style="padding: 18px; border-right: 1px solid var(--rule);">
          <TStat label="Size" :value="bytes(p.cache_stats.total_size)" size="sm" />
        </div>
        <div style="padding: 18px; border-right: 1px solid var(--rule);">
          <TStat label="Hits" :value="p.cache_stats.hit_count.toLocaleString()" size="sm" />
        </div>
        <div style="padding: 18px; border-right: 1px solid var(--rule);">
          <TStat label="Misses" :value="p.cache_stats.miss_count.toLocaleString()" size="sm" />
        </div>
        <div style="padding: 18px; border-right: 1px solid var(--rule);">
          <TStat label="Evictions" :value="p.cache_stats.eviction_count.toLocaleString()" size="sm" />
        </div>
        <div style="padding: 18px;">
          <TStat label="Hit Ratio" :value="pct(p.cache_stats.hit_ratio)" size="sm" />
        </div>
      </div>
      <p v-else-if="p.cache_stats" class="t-meta t-meta--warn" style="margin-bottom: 56px; padding: 14px; border: 1px solid var(--warning); border-radius: var(--t-radius); opacity: 0.85;">
        <code>{{ p.cache_stats.cache_type }}</code> backend does not implement Stats()
      </p>
    </template>

    <!-- RunnerConfig -->
    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">Configuration</h2>

    <dl class="t-dl">
      <dt>url</dt><dd><code>{{ p.url }}</code></dd>
      <dt>concurrency</dt><dd><code>{{ p.concurrency }}</code></dd>
      <dt>work_dir</dt><dd><code>{{ p.work_dir }}</code></dd>
      <template v-if="p.shell">
        <dt>shell</dt><dd><code>{{ p.shell }}</code></dd>
      </template>
      <dt>labels</dt>
      <dd>
        <span style="display: flex; flex-wrap: wrap; gap: 6px;">
          <TChip v-for="l in p.labels" :key="l">{{ l }}</TChip>
        </span>
      </dd>
      <dt>env</dt><dd><code>{{ p.environment_count }}</code> entries</dd>

      <template v-if="p.docker">
        <dt>docker.image</dt><dd><code>{{ p.docker.image }}</code></dd>
        <dt>docker.pull_policy</dt><dd><code>{{ p.docker.pull_policy }}</code></dd>
        <dt>docker.memory</dt><dd><code>{{ p.docker.memory }}</code></dd>
        <dt>docker.cpus</dt><dd><code>{{ p.docker.cpus }}</code></dd>
        <dt>docker.privileged</dt><dd><code>{{ p.docker.privileged }}</code></dd>
      </template>

      <template v-if="p.kubernetes">
        <dt>kubernetes.namespace</dt><dd><code>{{ p.kubernetes.namespace }}</code></dd>
        <dt>kubernetes.service_account</dt><dd><code>{{ p.kubernetes.service_account }}</code></dd>
        <dt>kubernetes.cpu_limit</dt><dd><code>{{ p.kubernetes.cpu_limit }}</code></dd>
        <dt>kubernetes.memory_limit</dt><dd><code>{{ p.kubernetes.memory_limit }}</code></dd>
      </template>
    </dl>
  </main>
</template>
