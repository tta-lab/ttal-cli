<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  words: string[]
  prefix?: string
  suffix?: string
}>()

const displayText = ref('')
let wordIndex = 0
let charIndex = 0
let isDeleting = false
let timeoutId: ReturnType<typeof setTimeout> | null = null

function tick() {
  const current = props.words[wordIndex]

  if (isDeleting) {
    charIndex--
  } else {
    charIndex++
  }

  displayText.value = current.substring(0, charIndex)

  let delay = isDeleting ? 50 : 100

  if (!isDeleting && charIndex === current.length) {
    delay = 2000
    isDeleting = true
  } else if (isDeleting && charIndex === 0) {
    isDeleting = false
    wordIndex = (wordIndex + 1) % props.words.length
    delay = 400
  }

  timeoutId = setTimeout(tick, delay)
}

onMounted(() => {
  if (props.words.length > 0) {
    tick()
  }
})

onUnmounted(() => {
  if (timeoutId !== null) {
    clearTimeout(timeoutId)
    timeoutId = null
  }
})
</script>

<template>
  <span class="typer-line">
    {{ prefix ?? 'The ' }}<span class="typer-word">{{ displayText }}</span><span class="typer-cursor">|</span>{{ suffix ?? ' Agentic Lab' }}
  </span>
</template>
