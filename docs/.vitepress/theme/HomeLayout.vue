<script setup>
import DefaultTheme from 'vitepress/theme'
import Typer from '../components/Typer.vue'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #home-hero-info-before>
      <div class="hero-typer">
        <Typer :words="['Telegram', 'Teamwork', 'Taskwarrior']" />
      </div>
      <p class="pronunciation">/tiː tɔːl/</p>
    </template>
  </Layout>
</template>
