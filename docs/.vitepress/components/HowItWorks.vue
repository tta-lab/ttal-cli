<template>
  <div class="how-it-works">
    <div class="steps">
      <div v-for="(step, index) in steps" :key="step.title" class="step-wrapper">
        <div class="step">
          <div class="step-icon">{{ step.icon }}</div>
          <div class="step-content">
            <div class="step-title">{{ step.title }}</div>
            <div class="step-desc">{{ step.desc }}</div>
            <div v-if="step.code" class="step-code">{{ step.code }}</div>
          </div>
        </div>
        <div v-if="index < steps.length - 1" class="step-arrow">→</div>
      </div>
    </div>
  </div>
</template>

<script setup>
const steps = [
  {
    icon: '📋',
    title: 'Task',
    desc: 'Define what to build',
    code: 'ttal task add --project api "Fix auth middleware"',
  },
  {
    icon: '🐙',
    title: 'Agent',
    desc: 'Agents research, design, and code',
    code: 'Research → Plan → Implement → Review',
  },
  {
    icon: '✅',
    title: 'PR',
    desc: 'Merged and done',
    code: 'Reviewer → triage → squash merge',
  },
]
</script>

<style scoped>
.how-it-works {
  margin: 2rem 0;
}

.steps {
  display: flex;
  align-items: flex-start;
  gap: 0;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .steps {
    flex-direction: column;
    gap: 0;
  }

  .step-arrow {
    transform: rotate(90deg);
    margin: 0.5rem auto !important;
  }
}

.step-wrapper {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 200px;
}

.step {
  flex: 1;
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-border);
  border-radius: 12px;
  padding: 1.5rem;
  text-align: center;
}

.step-icon {
  font-size: 2.5rem;
  margin-bottom: 0.75rem;
  line-height: 1;
}

.step-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin-bottom: 0.3rem;
}

.step-desc {
  font-size: 0.875rem;
  color: var(--vp-c-text-2);
  margin-bottom: 0.75rem;
}

.step-code {
  font-family: ui-monospace, 'SF Mono', 'Cascadia Code', monospace;
  font-size: 0.75rem;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-bg-mute);
  border-radius: 6px;
  padding: 0.4rem 0.6rem;
  white-space: pre;
  overflow: hidden;
  text-overflow: ellipsis;
}

.step-arrow {
  font-size: 1.5rem;
  color: var(--vp-c-text-3);
  padding: 0 0.75rem;
  flex-shrink: 0;
  margin-top: -1rem;
}
</style>
