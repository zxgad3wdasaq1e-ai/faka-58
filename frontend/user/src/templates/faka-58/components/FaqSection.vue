<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDown, Search } from 'lucide-vue-next'
import { useFaka58Copy } from '../siteCopy'
const c = useFaka58Copy()
const search = ref('')
const tab = ref('all')
const open = ref<number | null>(0)
const questions = computed(() => [
  { q: c.value.q1, a: c.value.a1, type: 'before' }, { q: c.value.q2, a: c.value.a2, type: 'after' },
  { q: c.value.q3, a: c.value.a3, type: 'before' }, { q: c.value.q4, a: c.value.a4, type: 'after' },
  { q: c.value.q5, a: c.value.a5, type: 'before' }, { q: c.value.q6, a: c.value.a6, type: 'before' },
])
const filtered = computed(() => questions.value.filter(x => (tab.value === 'all' || tab.value === x.type) && `${x.q}${x.a}`.toLowerCase().includes(search.value.toLowerCase().trim())))
</script>
<template>
  <section id="faq" class="faka-58-section faka-58-faq"><div class="faka-58-container"><div class="faka-58-section-heading"><span class="faka-58-section-tag">{{ c.faqTag }}</span><h2>{{ c.faqTitle }}</h2><p>{{ c.faqDesc }}</p></div><div class="faka-58-faq-content"><label class="faka-58-faq-search"><Search :size="18" /><input v-model="search" :placeholder="c.search" :aria-label="c.search" type="search" /></label><div class="faka-58-faq-tabs"><button v-for="item in [{ key: 'all', label: c.all }, { key: 'before', label: c.before }, { key: 'after', label: c.after }]" :key="item.key" :aria-pressed="tab === item.key" :class="{ active: tab === item.key }" @click="tab = item.key; open = null">{{ item.label }}</button></div><div class="faka-58-faq-list"><article v-for="(item, i) in filtered" :key="item.q" :class="{ expanded: open === i }"><button :aria-expanded="open === i" @click="open = open === i ? null : i"><h3>{{ item.q }}</h3><ChevronDown :size="18" /></button><p v-if="open === i">{{ item.a }}</p></article><p v-if="!filtered.length" class="faka-58-empty">{{ c.noResults }}</p></div><p class="faka-58-faq-bottom"><RouterLink to="/order">{{ c.orders }} →</RouterLink><RouterLink to="/guide">{{ c.guide }} →</RouterLink></p></div></div></section>
</template>
<style scoped>
.faka-58-faq-content{max-width:850px;margin:auto}.faka-58-faq-search{display:flex;gap:12px;align-items:center;padding:13px 19px;border:1px solid #e4e7f0;border-radius:13px;color:#a1a7ba;background:#fcfcfe}.faka-58-faq-search input{background:none;border:none;outline:none;color:#334155;font:inherit;font-size:14px;width:100%}.faka-58-faq-tabs{display:flex;justify-content:center;gap:10px;margin:24px 0}.faka-58-faq-tabs button{background:none;border:0;border-radius:20px;padding:8px 17px;font-size:13px;color:#79849b}.faka-58-faq-tabs button.active{background:#eeeaff;color:#6550ec;font-weight:600}.faka-58-faq-list article{border:1px solid #e5e8f2;border-radius:13px;margin:12px 0;overflow:hidden;transition:border-color .2s}.faka-58-faq-list article.expanded{border-color:#e1d9ff;background:#fdfcff}.faka-58-faq-list article>button{display:flex;justify-content:space-between;align-items:center;text-align:left;width:100%;padding:20px 22px;background:none;border:0;gap:20px}.faka-58-faq-list h3{font-size:15px;margin:0;font-weight:500}.faka-58-faq-list article>button>svg{color:#a8a0bc;flex-shrink:0;transition:transform .2s}.faka-58-faq-list article.expanded>button>svg{transform:rotate(180deg);color:#6c50eb}.faka-58-faq-list article>p{font-size:14px;line-height:1.9;color:#748096;padding:0 22px 22px;margin:0}.faka-58-faq-bottom{display:flex;justify-content:center;gap:30px;font-size:13px;color:#8b7ec4;margin-top:28px}@media(max-width:640px){.faka-58-faq-list article>button{padding:18px}.faka-58-faq-tabs{gap:0}.faka-58-faq-tabs button{padding:8px 13px}}
</style>
