<script setup>
import DefaultTheme from 'vitepress/theme'
import Typer from '../components/Typer.vue'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #home-hero-info-after>
      <Typer :words="['Telegram', 'Teamwork', 'Taskwarrior']" />
      <p class="vision-sub">
        One bot per agent. Coordinate your AI team from Telegram — voice, vision, and text.
      </p>
    </template>
  </Layout>
</template>
