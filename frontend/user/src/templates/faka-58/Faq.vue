<script setup lang="ts">
import FaqSection from './components/FaqSection.vue'
import { useFaka58Copy } from './siteCopy'
import { usePageSeo } from '../../composables/usePageSeo'
const c = useFaka58Copy()
usePageSeo({ title: () => c.value.faq })
</script>
<template><FaqSection /></template>
