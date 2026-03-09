<template>
  <div class="agent-roster">
    <div class="roster-grid">
      <div v-for="agent in agents" :key="agent.name" class="agent-card">
        <div class="agent-emoji">{{ agent.emoji }}</div>
        <div class="agent-name">{{ agent.name }}</div>
        <div class="agent-creature">{{ agent.creature }}</div>
        <div class="agent-role">{{ agent.role }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
const agents = [
  {
    emoji: '🐱',
    name: 'Yuki',
    creature: 'Cat',
    role: 'Orchestrator — routes tasks, coordinates the team, keeps things moving',
  },
  {
    emoji: '🦉',
    name: 'Athena',
    creature: 'Owl',
    role: 'Researcher — investigates, synthesizes, writes findings and plans',
  },
  {
    emoji: '🐙',
    name: 'Inke',
    creature: 'Octopus',
    role: 'Designer — writes implementation plans, shapes architecture decisions',
  },
  {
    emoji: '🦅',
    name: 'Kestrel',
    creature: 'Falcon',
    role: 'Bug-fix designer — diagnoses root causes and writes fix plans for workers to execute',
  },
]
</script>

<style scoped>
.agent-roster {
  margin: 2rem 0;
}

.roster-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
}

@media (max-width: 768px) {
  .roster-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .roster-grid {
    grid-template-columns: 1fr;
  }
}

.agent-card {
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-border);
  border-radius: 12px;
  padding: 1.5rem 1rem;
  text-align: center;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.agent-card:hover {
  border-color: var(--vp-c-brand-1);
  box-shadow: 0 4px 20px rgba(100, 108, 255, 0.15);
}

.agent-emoji {
  font-size: 2.5rem;
  margin-bottom: 0.75rem;
  line-height: 1;
}

.agent-name {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--vp-c-text-1);
  margin-bottom: 0.2rem;
}

.agent-creature {
  font-size: 0.8rem;
  color: var(--vp-c-brand-1);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: 0.6rem;
}

.agent-role {
  font-size: 0.8rem;
  color: var(--vp-c-text-2);
  line-height: 1.5;
}
</style>
