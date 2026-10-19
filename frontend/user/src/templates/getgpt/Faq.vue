<script setup lang="ts">
import FaqSection from './components/FaqSection.vue'
import { useGetgptCopy } from './siteCopy'
import { usePageSeo } from '../../composables/usePageSeo'
const c = useGetgptCopy()
usePageSeo({ title: () => c.value.faq })
</script>
<template><FaqSection /></template>
