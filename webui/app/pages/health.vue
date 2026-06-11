<script setup lang="ts">
const { data: snap } = await useFetch("/api/snapshot");

const healthz = computed(() => snap.value?.health.healthz);
const readyz  = computed(() => snap.value?.health.readyz);

const allOk = (m: Record<string, string> | undefined) =>
  !!m && Object.values(m).every(v => v === "ok");

const dot = (v: string) => v === "ok" ? "active" : "warn";
</script>

<template>
  <main v-if="snap" class="t-page t-page--wide">
    <header style="padding-bottom: 28px; border-bottom: 1px solid var(--rule); margin-bottom: 40px;">
      <TEyebrow bare>Probes · {{ snap.global.health_listen }}</TEyebrow>
      <TTitle size="md" style="margin-top: 12px;">Health</TTitle>
    </header>

    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">
      <code style="font-style: normal; color: var(--accent);">GET /healthz</code>
    </h2>

    <div style="border: 1px solid var(--rule); border-radius: var(--t-radius); padding: 18px 24px; margin-bottom: 32px;">
      <table class="t-table" style="border-top: none; border-bottom: none;">
        <tbody>
          <tr v-for="(value, key) in healthz!.checks" :key="key">
            <td style="width: 240px;"><span class="t-mono">{{ key }}</span></td>
            <td>
              <span style="display: inline-flex; align-items: center;">
                <TPipDot :state="dot(value)" />
                <span class="t-mono">{{ value }}</span>
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <h2 class="t-eyebrow t-eyebrow--bare" style="margin: 0 0 14px; justify-content: flex-start;">
      <code style="font-style: normal; color: var(--accent);">GET /readyz</code>
    </h2>

    <div style="border: 1px solid var(--rule); border-radius: var(--t-radius); padding: 18px 24px;">
      <table class="t-table" style="border-top: none; border-bottom: none;">
        <tbody>
          <tr v-for="(value, key) in readyz!.checks" :key="key">
            <td style="width: 240px;"><span class="t-mono">{{ key }}</span></td>
            <td>
              <span style="display: inline-flex; align-items: center;">
                <TPipDot :state="dot(value)" />
                <span class="t-mono">{{ value }}</span>
              </span>
            </td>
          </tr>
        </tbody>
      </table>

      <p v-if="readyz!.default_registry_empty"
         class="t-meta t-meta--warn"
         style="margin-top: 18px; padding: 12px 14px; border: 1px solid var(--warning); border-radius: var(--t-radius); opacity: 0.85;">
        only the <code>ready</code> key is wired by default · register
        <code>GitHubAPICheck</code> / <code>DiskSpaceCheck</code> /
        <code>ExecutorCheck</code> in <code>health.NewCheckRegistry()</code>
      </p>
    </div>
  </main>
</template>
