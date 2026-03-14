<template>
  <div class="stats-bar">
    <div class="stat">
      <span class="stat-number">356</span>
      <span class="stat-label">PRs merged</span>
    </div>
    <span class="stat-sep">·</span>
    <div class="stat">
      <span class="stat-number">29k</span>
      <span class="stat-label">lines of Go</span>
    </div>
    <span class="stat-sep">·</span>
    <div class="stat">
      <span class="stat-number">33</span>
      <span class="stat-label">days</span>
    </div>
    <span class="stat-sep">·</span>
    <div class="stat">
      <span class="stat-label stat-label--em">built with itself</span>
    </div>
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
