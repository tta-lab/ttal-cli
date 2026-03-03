<script setup lang="ts">
const plans = [
  {
    name: 'Free',
    price: '$0',
    period: 'forever',
    features: [
      'MIT license — full CLI, no gates',
      '1 team · 2 agents · unlimited coders',
      'Community support via GitHub',
      'Self-hosted on your machine',
    ],
    cta: 'Get Started',
    ctaLink: '/docs/getting-started',
    highlight: false,
  },
  {
    name: 'Pro',
    price: '$100',
    period: 'lifetime',
    features: [
      '1 team · unlimited agents · unlimited coders',
      'Direct access to the maintainer',
      'Priority bug fixes',
      'Your issues get attention first',
    ],
    cta: 'Buy Pro',
    ctaLink: '/pricing#payment',
    highlight: true,
  },
  {
    name: 'Team',
    price: '$200',
    period: 'lifetime',
    features: [
      'Unlimited teams · unlimited agents · unlimited coders',
      'Isolated project & agent registries per team',
      'Lifetime support for your organization',
      'Priority feature requests',
    ],
    cta: 'Buy Team',
    ctaLink: '/pricing#payment',
    highlight: false,
  },
]
</script>

<template>
  <div class="pricing-cards">
    <div
      v-for="plan in plans"
      :key="plan.name"
      class="pricing-card"
      :class="{ highlighted: plan.highlight }"
    >
      <div class="card-header">
        <h3>{{ plan.name }}</h3>
        <div class="price">{{ plan.price }}</div>
        <div class="period">{{ plan.period }}</div>
      </div>
      <ul class="features">
        <li v-for="f in plan.features" :key="f">{{ f }}</li>
      </ul>
      <a :href="plan.ctaLink" class="cta-btn" :class="{ primary: plan.highlight }">
        {{ plan.cta }}
      </a>
    </div>
  </div>
</template>

<style scoped>
.pricing-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.5rem;
  margin: 2rem auto;
  max-width: 54rem;
}

@media (max-width: 768px) {
  .pricing-cards {
    grid-template-columns: 1fr;
  }
}

.pricing-card {
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  padding: 2rem 1.5rem;
  display: flex;
  flex-direction: column;
  background: var(--vp-c-bg);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.pricing-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.pricing-card.highlighted {
  border-color: var(--vp-c-brand-1);
  box-shadow: 0 0 0 1px var(--vp-c-brand-1);
}

.card-header {
  text-align: center;
  margin-bottom: 1.5rem;
}

.card-header h3 {
  margin: 0 0 0.5rem;
  font-size: 1.2rem;
}

.price {
  font-size: 2.5rem;
  font-weight: 700;
  line-height: 1;
  color: var(--vp-c-text-1);
}

.period {
  font-size: 0.9rem;
  color: var(--vp-c-text-3);
  margin-top: 0.25rem;
}

.features {
  list-style: none;
  padding: 0;
  margin: 0 0 1.5rem;
  flex: 1;
}

.features li {
  padding: 0.4rem 0;
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
  border-bottom: 1px solid var(--vp-c-divider-light);
}

.features li:last-child {
  border-bottom: none;
}

.cta-btn {
  display: block;
  text-align: center;
  padding: 0.75rem 1.5rem;
  border-radius: 8px;
  font-weight: 600;
  text-decoration: none;
  border: 1px solid var(--vp-c-divider);
  color: var(--vp-c-text-1);
  transition: background 0.15s ease;
}

.cta-btn:hover {
  background: var(--vp-c-bg-soft);
}

.cta-btn.primary {
  background: var(--vp-c-brand-1);
  color: var(--vp-c-white);
  border-color: var(--vp-c-brand-1);
}

.cta-btn.primary:hover {
  background: var(--vp-c-brand-2);
}
</style>
