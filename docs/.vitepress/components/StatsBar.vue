<script setup>
const props = withDefaults(defineProps<{
  stats?: Array<{ number?: string; label: string; em?: boolean }>
}>(), {
  stats: () => [
    { number: '356', label: 'PRs merged' },
    { number: '29k', label: 'lines of Go' },
    { number: '33',  label: 'days' },
    { label: 'built with itself', em: true },
  ],
})
</script>

<template>
  <div class="stats-bar">
    <template v-for="(stat, i) in props.stats" :key="i">
      <span v-if="i > 0" class="stat-sep">·</span>
      <div class="stat">
        <span v-if="stat.number" class="stat-number">{{ stat.number }}</span>
        <span class="stat-label" :class="{ 'stat-label--em': stat.em }">{{ stat.label }}</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.stats-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 1rem 0 2rem;
  flex-wrap: wrap;
}

.stat {
  display: flex;
  align-items: baseline;
  gap: 0.35rem;
}

.stat-number {
  font-size: 1.4rem;
  font-weight: 700;
  color: var(--vp-c-brand-1);
  font-variant-numeric: tabular-nums;
}

.stat-label {
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
}

.stat-label--em {
  font-style: italic;
}

.stat-sep {
  color: var(--vp-c-divider);
  font-size: 1.2rem;
  line-height: 1;
}
</style>
