<template>
  <div class="home-shell">
    <div class="mesh" aria-hidden="true"></div>

    <div class="nav-shell">
      <header class="nav">
        <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="brand">
          <span class="brand-mark">
            <img v-if="siteLogo" :src="siteLogo" alt="Logo" />
          </span>
          <span>{{ siteName }}</span>
        </a>

        <ul>
          <li><a href="#mission">{{ t('home.landing.nav.mission') }}</a></li>
          <li><a href="#workflow">{{ t('home.landing.nav.workflow') }}</a></li>
          <li><a href="#skills">{{ t('home.landing.nav.skills') }}</a></li>
          <li><a href="#infrastructure">{{ t('home.landing.nav.infrastructure') }}</a></li>
        </ul>

        <div class="nav-actions">
          <LocaleSwitcher />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="sm" />
          </a>

          <button
            class="icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>

          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="pill">
            <span>{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</span>
            <span class="icon-wrap">
              <Icon :name="isAuthenticated ? 'arrowRight' : 'login'" size="xs" />
            </span>
          </router-link>
        </div>
      </header>
    </div>

    <main>
      <section class="wrap hero">
        <div class="hero-copy">
          <span class="eyebrow">
            <span class="dot"></span>
            {{ t('home.landing.badge.status') }} · {{ t('home.landing.badge.text') }}
          </span>

          <h1 class="hero-title">
            <span class="hero-title-main">{{ t('home.landing.titleMain') }}</span>
            <span class="hero-title-row">
              <span class="hero-title-connector">{{ t('home.landing.titleConnector') }}</span>
            </span>
            <span class="hero-title-field">
              <span class="research-field-roller" :aria-label="researchFieldsLabel">
                <span class="research-field-track" aria-hidden="true">
                  <span
                    v-for="(field, index) in researchFields"
                    :key="`${field}-${index}`"
                    class="research-field-item"
                  >
                    {{ field }}
                  </span>
                </span>
              </span>
            </span>
            <span class="soft">{{ t('home.landing.heroSoft') }}</span>
          </h1>

          <p class="lede">{{ t('home.landing.description') }}</p>

          <div class="actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="pill large">
              <span>{{ primaryActionLabel }}</span>
              <span class="icon-wrap">
                <Icon name="arrowRight" size="xs" />
              </span>
            </router-link>

            <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="ghost">
              <Icon name="externalLink" size="sm" />
              <span>{{ t('home.viewOnGithub') }}</span>
              <span v-if="githubStarsLabel" class="github-stars">{{ githubStarsLabel }}</span>
            </a>
          </div>

          <div class="hero-signal-row" aria-label="Research platform signals">
            <div
              v-for="(signal, index) in heroSignals"
              :key="signal.label"
              :class="['hero-signal', { 'hero-signal-primary': index === 0 }]"
            >
              <i>{{ String(index + 1).padStart(2, '0') }}</i>
              <span>{{ signal.label }}</span>
              <strong>{{ signal.value }}</strong>
              <p>{{ signal.description }}</p>
            </div>
          </div>
        </div>

        <aside class="hero-preview runtime-preview" aria-label="Research runtime preview">
          <div class="core">
            <div class="runtime-topbar">
              <div class="runtime-window">
                <div class="preview-bar"><span></span><span></span><span></span></div>
                <b>{{ t('home.landing.mockupLabel') }}</b>
              </div>
              <span class="live-chip"><span class="pulse"></span>{{ t('home.landing.preview.live') }}</span>
            </div>

            <div class="runtime-head">
              <div>
                <div class="meta">{{ t('home.landing.preview.researchLabel') }}</div>
                <h3>{{ t('home.landing.preview.researchTitle') }}</h3>
                <p>{{ t('home.landing.preview.researchRoute') }}</p>
              </div>
              <div class="runtime-radial" aria-hidden="true">
                <strong>{{ workflowSteps.length }}/5</strong>
                <span>{{ t('home.landing.preview.progressLabel') }}</span>
              </div>
            </div>

            <div class="runtime-stage-map" aria-label="Research loop stages">
              <div
                v-for="(step, index) in workflowSteps"
                :key="step.title"
                :class="['runtime-stage', { active: index === 2 }]"
              >
                <span>{{ String(index + 1).padStart(2, '0') }}</span>
                <strong>{{ step.title }}</strong>
              </div>
            </div>

            <div class="runtime-body-grid">
              <div class="runtime-card runtime-card-dark">
                <div class="meta">{{ t('home.landing.preview.skillSignal') }}</div>
                <strong>{{ t('home.landing.skillMarquee.signals.workflow.value') }}</strong>
                <p>{{ t('home.landing.skillMarquee.signals.workflow.description') }}</p>
              </div>

              <div class="runtime-card">
                <div class="meta">{{ t('home.landing.preview.resourceSignal') }}</div>
                <strong>Token / GPU</strong>
                <p>{{ t('home.landing.infrastructure.cards.gpu.description') }}</p>
                <div class="runtime-meter" aria-hidden="true"><span></span></div>
              </div>

              <a
                :href="githubUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="runtime-card runtime-card-link"
              >
                <div class="meta">{{ t('home.landing.preview.githubSignal') }}</div>
                <strong>{{ githubStarsLabel || t('home.landing.skillMarquee.github.value') }}</strong>
                <p>{{ t('home.landing.skillMarquee.github.description') }}</p>
              </a>
            </div>

            <div class="runtime-trace">
              <div class="meta">{{ t('home.landing.preview.traceLabel') }}</div>
              <p>
                <code>04:12:09</code>
                <span>{{ t('home.landing.preview.traceOne') }}</span>
                <span>{{ t('home.landing.preview.traceTwo') }}</span>
                <b>{{ t('home.landing.preview.traceThree') }}</b>
              </p>
            </div>
          </div>
        </aside>
      </section>

      <section id="skills" class="skills-section">
        <div class="wrap skill-radar">
          <div class="skill-radar-copy">
            <span class="eyebrow"><span class="dot"></span>{{ t('home.landing.skillMarquee.badge') }}</span>
            <h2>{{ t('home.landing.skillMarquee.title') }}</h2>
            <p>{{ t('home.landing.skillMarquee.description') }}</p>
          </div>

          <div class="skill-radar-board">
            <article
              v-for="signal in skillSignals"
              :key="signal.label"
              class="signal-card"
            >
              <span>{{ signal.label }}</span>
              <strong>{{ signal.value }}</strong>
              <p>{{ signal.description }}</p>
            </article>

            <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="signal-card signal-card-dark">
              <span>{{ t('home.landing.skillMarquee.github.label') }}</span>
              <strong>{{ githubStarsLabel || t('home.landing.skillMarquee.github.value') }}</strong>
              <p>
                {{
                  githubStarsLabel
                    ? t('home.landing.skillMarquee.github.starsDescription')
                    : t('home.landing.skillMarquee.github.description')
                }}
              </p>
            </a>
          </div>
        </div>

        <div class="marquee" aria-label="Research skill repositories">
          <div class="marquee-track">
            <a
              v-for="(repo, index) in marqueeSkillRepos"
              :key="`${repo.url}-${index}`"
              :href="repo.url"
              target="_blank"
              rel="noopener noreferrer"
              class="marquee-item"
            >
              {{ repo.name }}<em>·</em>
            </a>
          </div>
        </div>
      </section>

      <section id="mission" class="block">
        <div class="wrap">
          <div class="section-head">
            <div>
              <span class="eyebrow"><span class="dot"></span>{{ t('home.landing.mission.badge') }}</span>
              <h2>{{ t('home.landing.mission.title') }}</h2>
            </div>
            <p>{{ t('home.landing.mission.description') }}</p>
          </div>

          <div class="bento">
            <article class="b-card span-3 row-2">
              <div class="core">
                <div class="meta">{{ t('home.landing.mission.operatingBeliefLabel') }}</div>
                <h3>{{ t('home.landing.mission.operatingBelief') }}</h3>
                <p>{{ t('home.landing.capabilities.description') }}</p>
                <div class="mission-metrics">
                  <div
                    v-for="metric in missionMetrics"
                    :key="metric.label"
                    class="mission-metric"
                  >
                    <span>{{ metric.label }}</span>
                    <strong>{{ metric.value }}</strong>
                    <p>{{ metric.description }}</p>
                  </div>
                </div>
                <div class="tag-row">
                  <span v-for="tag in missionTags" :key="tag">{{ tag }}</span>
                </div>
              </div>
            </article>

            <article
              v-for="(principle, index) in missionPrinciples"
              :key="principle.title"
              :class="['b-card', index === 1 ? 'span-3 dark' : 'span-3']"
            >
              <div class="core">
                <div class="meta">{{ String(index + 1).padStart(2, '0') }} · mission</div>
                <h3>{{ principle.title }}</h3>
                <p>{{ principle.description }}</p>
              </div>
            </article>
          </div>
        </div>
      </section>

      <section id="workflow" class="block">
        <div class="wrap">
          <div class="section-head">
            <div>
              <span class="eyebrow"><span class="dot"></span>{{ t('home.landing.workflow.badge') }}</span>
              <h2>{{ t('home.landing.workflow.title') }}</h2>
            </div>
            <p>{{ t('home.landing.workflow.description') }}</p>
          </div>

          <div class="bento">
            <article
              v-for="(step, index) in workflowSteps"
              :key="step.title"
              :class="['b-card', index < 2 ? 'span-3' : index === 2 ? 'span-2 dark' : 'span-2']"
            >
              <div class="core">
                <div class="meta">{{ String(index + 1).padStart(2, '0') }} · workflow</div>
                <h3>{{ step.title }}</h3>
                <p>{{ step.description }}</p>
              </div>
            </article>
          </div>
        </div>
      </section>

      <section id="infrastructure" class="block">
        <div class="wrap">
          <div class="section-head">
            <div>
              <span class="eyebrow"><span class="dot"></span>{{ t('home.landing.infrastructure.badge') }}</span>
              <h2>{{ t('home.landing.infrastructure.title') }}</h2>
            </div>
            <p>{{ t('home.landing.infrastructure.description') }}</p>
          </div>

          <div class="bento">
            <article
              v-for="(card, index) in infrastructureCards"
              :key="card.title"
              :class="['b-card', index === 0 ? 'span-3 dark' : index === 1 ? 'span-3' : 'span-2']"
            >
              <div class="core">
                <div class="meta">{{ String(index + 1).padStart(2, '0') }} · infra</div>
                <h3>{{ card.title }}</h3>
                <p>{{ card.description }}</p>
              </div>
            </article>

            <article class="b-card span-2">
              <div class="core">
                <div class="meta">{{ t('home.landing.preview.costLabel') }}</div>
                <h3>$0.018</h3>
                <p>{{ t('home.landing.preview.route') }}</p>
              </div>
            </article>
            <article class="b-card span-2">
              <div class="core">
                <div class="meta">{{ t('home.landing.preview.latencyLabel') }}</div>
                <h3>412ms</h3>
                <p>{{ t('home.landing.preview.traceTwo') }}</p>
              </div>
            </article>
          </div>
        </div>
      </section>

      <section class="closing">
        <div class="closing-inner">
          <h2>{{ t('home.landing.closing.title') }}</h2>
          <p>{{ t('home.landing.closing.description') }}</p>
          <div class="closing-actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="pill">
              <span>{{ primaryActionLabel }}</span>
              <span class="icon-wrap">
                <Icon name="arrowRight" size="xs" />
              </span>
            </router-link>
            <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="ghost invert">
              {{ t('home.viewOnGithub') }}
            </a>
          </div>
        </div>
      </section>
    </main>

    <footer>
      <div class="footer-row">
        <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
        <span>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
          <span v-if="docUrl"> · </span>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
        </span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

defineProps<{
  siteName: string
  siteLogo: string
  docUrl: string
}>()

const authStore = useAuthStore()

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))
const githubStars = ref<number | null>(null)

// GitHub URL
const githubUrl = 'https://github.com/wanshuiyin/Auto-claude-code-research-in-sleep'
const githubRepoApiUrl = 'https://api.github.com/repos/wanshuiyin/Auto-claude-code-research-in-sleep'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

const primaryActionLabel = computed(() =>
  isAuthenticated.value ? t('home.goToDashboard') : t('home.getStarted')
)

const githubStarsLabel = computed(() => {
  if (githubStars.value === null) return ''
  return `${formatCompactNumber(githubStars.value)} stars`
})

const researchFields = computed(() => [
  t('home.landing.researchFields.machineLearning'),
  t('home.landing.researchFields.dataScience'),
  t('home.landing.researchFields.computerScience'),
  t('home.landing.researchFields.artificialIntelligence'),
  t('home.landing.researchFields.neuralLanguageProcessing'),
  t('home.landing.researchFields.computerVision'),
  t('home.landing.researchFields.machineLearning')
])

const researchFieldsLabel = computed(() => t('home.landing.researchFieldsLabel'))

const skillRepos = [
  { name: 'Scientific Agent Skills', url: 'https://github.com/K-Dense-AI/scientific-agent-skills' },
  { name: 'Nature Skills', url: 'https://github.com/Yuan1z0825/nature-skills' },
  { name: 'Nature Paper Skills', url: 'https://github.com/Boom5426/Nature-Paper-Skills' },
  { name: 'ARIS Auto Research', url: 'https://github.com/wanshuiyin/Auto-claude-code-research-in-sleep' },
  { name: 'Dr. Claw', url: 'https://github.com/OpenLAIR/dr-claw' },
  { name: 'SciAgent Skills', url: 'https://github.com/jaechang-hits/SciAgent-Skills' },
  { name: 'Codex Claude Academic Skills', url: 'https://github.com/zLanqing/codex-claude-academic-skills' },
  { name: 'AI Research Skills', url: 'https://github.com/WenyuChiou/ai-research-skills' },
  { name: 'PaperSprint', url: 'https://github.com/RichradsY/PaperSprint' },
  { name: 'Manuscript Writing', url: 'https://github.com/YSLAB-ai/manuscript-writing' },
  { name: 'Research Paper Writing Skills', url: 'https://github.com/Brandlytic/Research-Paper-Writing-Skills' },
  { name: 'SciWrite', url: 'https://github.com/labarba/sciwrite' },
  { name: 'Paper Writer Skill', url: 'https://github.com/kgraph57/paper-writer-skill' },
  { name: 'Research Papers Skill', url: 'https://github.com/marciob/skill-research-papers' },
  { name: 'Engineering Figure Agent', url: 'https://github.com/heyu-233/engineering-figure-agent' },
  { name: 'Thesis Defense PPTX Skill', url: 'https://github.com/zouchenzhen/thesis-defense-pptx-skill' },
  { name: 'Academic PPT Skill', url: 'https://github.com/PHY041/claude-skill-academic-ppt' },
  { name: 'Academic PPTX Skill', url: 'https://github.com/Gabberflast/academic-pptx-skill' },
  { name: 'Chinese Reference Formatter', url: 'https://github.com/Zechang-Xiong/chinese-reference-formatter-skill' },
  { name: 'Codex Academic Skills', url: 'https://github.com/Epsilon617/Codex-Academic-Skills' },
  { name: 'Awesome Scientific Skills', url: 'https://github.com/InternScience/Awesome-Scientific-Skills' },
  { name: 'AI Research SKILLs', url: 'https://github.com/zechenzhangAGI/AI-research-SKILLs' },
  { name: 'Scientific Thinking General', url: 'https://github.com/Agents365-ai/scientific-thinking-general' },
  { name: 'SciComp Research Skills', url: 'https://github.com/a-attia/scicomp-research-skills' },
  { name: 'SCI Writing', url: 'https://github.com/Eroticoo/sci-writing' },
  { name: 'OR/MS Writing Skill', url: 'https://github.com/jmf-enigma/or-ms-writing-skill' },
  { name: 'SCI Introduction', url: 'https://github.com/stephenlzc/sci-introduction' },
  { name: 'Academic Style Rewriter', url: 'https://github.com/AlimuratYusup/Academic-Style-Rewriter' },
  { name: 'PaperForge', url: 'https://github.com/Earl000333/paperforge' },
  { name: 'Modeling Skills Git', url: 'https://github.com/halsun2048/modeling-skills-git' }
] as const

const marqueeSkillRepos = computed(() => [...skillRepos, ...skillRepos])

const skillSignals = computed(() => [
  {
    label: t('home.landing.skillMarquee.signals.workflow.label'),
    value: t('home.landing.skillMarquee.signals.workflow.value'),
    description: t('home.landing.skillMarquee.signals.workflow.description')
  },
  {
    label: t('home.landing.skillMarquee.signals.writing.label'),
    value: t('home.landing.skillMarquee.signals.writing.value'),
    description: t('home.landing.skillMarquee.signals.writing.description')
  },
  {
    label: t('home.landing.skillMarquee.signals.execution.label'),
    value: t('home.landing.skillMarquee.signals.execution.value'),
    description: t('home.landing.skillMarquee.signals.execution.description')
  }
])

const heroSignals = computed(() => [
  {
    label: t('home.landing.heroSignals.loop.label'),
    value: t('home.landing.heroSignals.loop.value'),
    description: t('home.landing.heroSignals.loop.description')
  },
  {
    label: t('home.landing.heroSignals.agent.label'),
    value: t('home.landing.heroSignals.agent.value'),
    description: t('home.landing.heroSignals.agent.description')
  },
  {
    label: t('home.landing.heroSignals.infra.label'),
    value: t('home.landing.heroSignals.infra.value'),
    description: t('home.landing.heroSignals.infra.description')
  }
])

const missionTags = computed(() => [
  t('home.landing.mission.tags.lowFriction'),
  t('home.landing.mission.tags.researcherLed'),
  t('home.landing.mission.tags.thoughtSpeed')
])

const missionMetrics = computed(() => [
  {
    label: t('home.landing.mission.metrics.loop.label'),
    value: t('home.landing.mission.metrics.loop.value'),
    description: t('home.landing.mission.metrics.loop.description')
  },
  {
    label: t('home.landing.mission.metrics.assets.label'),
    value: t('home.landing.mission.metrics.assets.value'),
    description: t('home.landing.mission.metrics.assets.description')
  },
  {
    label: t('home.landing.mission.metrics.role.label'),
    value: t('home.landing.mission.metrics.role.value'),
    description: t('home.landing.mission.metrics.role.description')
  },
  {
    label: t('home.landing.mission.metrics.infra.label'),
    value: t('home.landing.mission.metrics.infra.value'),
    description: t('home.landing.mission.metrics.infra.description')
  }
])

const missionPrinciples = computed(() => [
  {
    title: t('home.landing.mission.principles.friction.title'),
    description: t('home.landing.mission.principles.friction.description')
  },
  {
    title: t('home.landing.mission.principles.judgment.title'),
    description: t('home.landing.mission.principles.judgment.description')
  },
  {
    title: t('home.landing.mission.principles.taste.title'),
    description: t('home.landing.mission.principles.taste.description')
  }
])

const workflowSteps = computed(() => [
  {
    title: t('home.landing.workflow.steps.literature.title'),
    description: t('home.landing.workflow.steps.literature.description')
  },
  {
    title: t('home.landing.workflow.steps.ideation.title'),
    description: t('home.landing.workflow.steps.ideation.description')
  },
  {
    title: t('home.landing.workflow.steps.experiment.title'),
    description: t('home.landing.workflow.steps.experiment.description')
  },
  {
    title: t('home.landing.workflow.steps.analysis.title'),
    description: t('home.landing.workflow.steps.analysis.description')
  },
  {
    title: t('home.landing.workflow.steps.writing.title'),
    description: t('home.landing.workflow.steps.writing.description')
  }
])

const infrastructureCards = computed(() => [
  {
    title: t('home.landing.infrastructure.cards.gateway.title'),
    description: t('home.landing.infrastructure.cards.gateway.description')
  },
  {
    title: t('home.landing.infrastructure.cards.token.title'),
    description: t('home.landing.infrastructure.cards.token.description')
  },
  {
    title: t('home.landing.infrastructure.cards.billing.title'),
    description: t('home.landing.infrastructure.cards.billing.description')
  },
  {
    title: t('home.landing.infrastructure.cards.gpu.title'),
    description: t('home.landing.infrastructure.cards.gpu.description')
  }
])

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

function formatCompactNumber(value: number): string {
  if (value >= 1000000) {
    return `${(value / 1000000).toFixed(value >= 10000000 ? 0 : 1)}M`
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(value >= 10000 ? 0 : 1)}k`
  }
  return String(value)
}

async function fetchGithubStars() {
  try {
    const response = await fetch(githubRepoApiUrl, {
      headers: {
        Accept: 'application/vnd.github+json'
      }
    })

    if (!response.ok) return

    const payload = await response.json() as { stargazers_count?: unknown }
    if (typeof payload.stargazers_count === 'number') {
      githubStars.value = payload.stargazers_count
    }
  } catch {
    // Public GitHub API can be rate-limited or blocked; the page works without the count.
  }
}

onMounted(() => {
  initTheme()
  fetchGithubStars()

  // Check auth state
  authStore.checkAuth()
})

</script>

<style scoped>
.home-shell {
  --canvas: #f2f2f0;
  --canvas-2: #e8e8e5;
  --ink: #0a0a0a;
  --muted: #57575a;
  --hairline: rgba(10, 10, 10, 0.08);
  --hairline-strong: rgba(10, 10, 10, 0.12);
  --innerlight: rgba(255, 255, 255, 0.65);
  --accent: #1f4d3f;
  --accent-glow: #5a8c7a;
  --accent-soft: rgba(31, 77, 63, 0.08);
  --sand: #c9b79a;
  --ease: cubic-bezier(0.32, 0.72, 0, 1);
  --ease-spring: cubic-bezier(0.16, 1.16, 0.3, 1);
  --shell-radius: 2.25rem;
  --core-radius: calc(2.25rem - 0.4rem);
  position: relative;
  min-height: 100vh;
  overflow-x: hidden;
  background: var(--canvas);
  color: var(--ink);
  font-family:
    'Plus Jakarta Sans',
    'Geist',
    'Cabinet Grotesk',
    system-ui,
    sans-serif;
  letter-spacing: 0;
  isolation: isolate;
}

:global(.dark) .home-shell {
  --canvas: #10110f;
  --canvas-2: #171917;
  --ink: #f4f4ef;
  --muted: rgba(244, 244, 239, 0.64);
  --hairline: rgba(255, 255, 255, 0.1);
  --hairline-strong: rgba(255, 255, 255, 0.16);
  --innerlight: rgba(255, 255, 255, 0.12);
  --accent: #8fcfb7;
  --accent-glow: #4da084;
  --accent-soft: rgba(143, 207, 183, 0.12);
  --sand: #d8c5a7;
}

.home-shell::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(10, 10, 10, 0.028) 1px, transparent 1px),
    linear-gradient(90deg, rgba(10, 10, 10, 0.028) 1px, transparent 1px);
  background-size: 74px 74px;
  mask-image: linear-gradient(to bottom, black 0%, transparent 76%);
}

:global(.dark) .home-shell::before {
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.055) 1px, transparent 1px);
}

.wrap {
  max-width: 1320px;
  margin: 0 auto;
  padding: 0 28px;
}

.mesh {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}

.mesh::before,
.mesh::after {
  content: '';
  position: absolute;
  border-radius: 9999px;
  filter: blur(90px);
  opacity: 0.18;
  will-change: transform;
}

.mesh::before {
  width: 760px;
  height: 760px;
  background: radial-gradient(circle at 30% 30%, var(--accent-glow), transparent 60%);
  top: -180px;
  left: -160px;
  animation: drift1 28s var(--ease) infinite alternate;
}

.mesh::after {
  width: 680px;
  height: 680px;
  background: radial-gradient(circle at 60% 60%, var(--sand), transparent 60%);
  top: 200px;
  right: -180px;
  animation: drift2 36s var(--ease) infinite alternate;
}

@keyframes drift1 {
  from {
    transform: translate3d(0, 0, 0);
  }
  to {
    transform: translate3d(80px, 60px, 0);
  }
}

@keyframes drift2 {
  from {
    transform: translate3d(0, 0, 0);
  }
  to {
    transform: translate3d(-60px, 40px, 0);
  }
}

main,
.nav-shell,
footer {
  position: relative;
  z-index: 1;
}

.nav-shell {
  position: sticky;
  top: 24px;
  z-index: 50;
  width: max-content;
  max-width: calc(100% - 56px);
  margin: 24px auto 0;
  padding: 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.42);
  backdrop-filter: saturate(160%) blur(22px);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.45) inset,
    0 0 0 1px rgba(10, 10, 10, 0.05),
    0 12px 36px -18px rgba(10, 10, 10, 0.18);
}

:global(.dark) .nav-shell {
  background: rgba(24, 24, 20, 0.5);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.08) inset,
    0 0 0 1px rgba(255, 255, 255, 0.08),
    0 12px 36px -18px rgba(0, 0, 0, 0.6);
}

.nav {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 6px 8px 6px 18px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.55);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.6);
}

:global(.dark) .nav {
  background: rgba(255, 255, 255, 0.06);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--ink);
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0;
  text-decoration: none;
  white-space: nowrap;
}

.brand-mark {
  display: inline-flex;
  width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 8px;
  background: linear-gradient(135deg, var(--accent) 0%, var(--accent-glow) 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.35),
    0 1px 2px rgba(10, 10, 10, 0.18);
}

.brand-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.nav ul {
  display: flex;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.nav ul a {
  display: inline-block;
  padding: 9px 14px;
  border-radius: 999px;
  color: var(--ink);
  font-size: 13px;
  font-weight: 500;
  text-decoration: none;
  transition: background 300ms var(--ease);
}

.nav ul a:hover {
  background: rgba(10, 10, 10, 0.05);
}

:global(.dark) .nav ul a:hover {
  background: rgba(255, 255, 255, 0.08);
}

.nav-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.icon-btn {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  transition:
    background 300ms var(--ease),
    color 300ms var(--ease);
}

.icon-btn:hover {
  background: rgba(10, 10, 10, 0.05);
  color: var(--ink);
}

:global(.dark) .icon-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

.pill,
.ghost {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 999px;
  text-decoration: none;
  white-space: nowrap;
  cursor: pointer;
  transition:
    transform 300ms var(--ease),
    background 300ms var(--ease),
    color 300ms var(--ease);
}

.pill {
  gap: 6px;
  padding: 7px 7px 7px 16px;
  background: var(--ink);
  color: var(--canvas);
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.12);
}

.pill.large {
  padding: 9px 9px 9px 20px;
  font-size: 14px;
}

.pill .icon-wrap {
  display: inline-flex;
  width: 26px;
  height: 26px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.14);
  transition:
    transform 360ms var(--ease-spring),
    background 300ms var(--ease);
}

.pill:hover .icon-wrap {
  transform: translate(2px, -1px) scale(1.06);
  background: rgba(255, 255, 255, 0.22);
}

.pill:active,
.ghost:active {
  transform: scale(0.98);
}

.ghost {
  gap: 8px;
  padding: 12px 18px;
  background: rgba(255, 255, 255, 0.5);
  color: var(--ink);
  box-shadow: inset 0 0 0 1px var(--hairline-strong);
  font-size: 13.5px;
  font-weight: 600;
}

.ghost:hover {
  background: rgba(255, 255, 255, 0.85);
}

.github-stars {
  margin-left: 2px;
  border-left: 1px solid var(--hairline-strong);
  padding-left: 10px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
}

.hero {
  min-height: min(780px, calc(100vh - 140px));
  display: grid;
  grid-template-columns: minmax(500px, 1fr) minmax(520px, 0.92fr);
  gap: clamp(46px, 5.2vw, 84px);
  align-items: center;
  padding-top: clamp(44px, 6vh, 68px);
  padding-bottom: clamp(54px, 7vh, 86px);
}

.hero-copy {
  position: relative;
}

.hero-copy::before {
  content: '';
  position: absolute;
  top: -34px;
  left: -28px;
  z-index: -1;
  width: min(560px, 88vw);
  height: 520px;
  border-radius: 34px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.32), transparent 46%),
    repeating-linear-gradient(
      135deg,
      rgba(31, 77, 63, 0.05) 0,
      rgba(31, 77, 63, 0.05) 1px,
      transparent 1px,
      transparent 18px
    );
  opacity: 0.74;
  mask-image: linear-gradient(135deg, black 0%, transparent 72%);
}

.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.6);
  box-shadow: inset 0 0 0 1px var(--hairline);
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    'Geist Mono',
    ui-monospace,
    monospace;
  font-size: 10px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

:global(.dark) .eyebrow {
  background: rgba(255, 255, 255, 0.07);
}

.eyebrow .dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--accent);
  box-shadow: 0 0 0 4px rgba(31, 77, 63, 0.18);
}

.hero-title {
  display: flex;
  max-width: 100%;
  flex-direction: column;
  gap: 0;
  margin: 22px 0 28px;
  color: var(--ink);
  font-size: clamp(58px, 5.8vw, 96px);
  font-weight: 700;
  line-height: 0.92;
  letter-spacing: 0;
}

.hero-title-main {
  display: block;
  overflow-wrap: normal;
}

.hero-title-row {
  display: block;
}

.hero-title-connector {
  display: block;
  color: var(--ink);
  white-space: nowrap;
}

.hero-title-field {
  display: block;
  width: 100%;
}

.hero-title .soft {
  display: block;
  color: var(--muted);
  font-weight: 500;
  line-height: 0.98;
}

.research-field-roller {
  display: block;
  width: min(8.4ch, 100%);
  height: 1.08em;
  overflow: hidden;
  line-height: 1.08;
}

.research-field-track {
  display: flex;
  flex-direction: column;
  will-change: transform;
  animation: tokenresearch-research-field-roll 18s cubic-bezier(0.76, 0, 0.24, 1) infinite;
}

.research-field-item {
  display: block;
  height: 1.08em;
  white-space: nowrap;
  color: var(--accent);
  line-height: 1.08;
}

@keyframes tokenresearch-research-field-roll {
  0%,
  10% {
    transform: translateY(0);
  }
  14%,
  24% {
    transform: translateY(-1.08em);
  }
  28%,
  38% {
    transform: translateY(-2.16em);
  }
  42%,
  52% {
    transform: translateY(-3.24em);
  }
  56%,
  66% {
    transform: translateY(-4.32em);
  }
  70%,
  80% {
    transform: translateY(-5.4em);
  }
  84%,
  100% {
    transform: translateY(-6.48em);
  }
}

.lede {
  max-width: 52ch;
  margin: 0 0 30px;
  color: var(--muted);
  font-size: 18px;
  line-height: 1.55;
}

.hero-signal-row {
  display: grid;
  max-width: 680px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 28px;
  padding: 6px;
  border-radius: 1.35rem;
  background: rgba(255, 255, 255, 0.34);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.56);
}

:global(.dark) .hero-signal-row {
  background: rgba(255, 255, 255, 0.045);
}

.hero-signal {
  position: relative;
  min-height: 96px;
  border-radius: 1rem;
  background: transparent;
  box-shadow: inset -1px 0 0 var(--hairline);
  padding: 14px 14px 14px 16px;
}

.hero-signal:last-child {
  box-shadow: none;
}

.hero-signal-primary {
  background: rgba(255, 255, 255, 0.58);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.68),
    0 18px 32px -28px rgba(10, 10, 10, 0.28);
}

:global(.dark) .hero-signal {
  background: transparent;
}

:global(.dark) .hero-signal-primary {
  background: rgba(255, 255, 255, 0.08);
}

.hero-signal i {
  position: absolute;
  top: 14px;
  right: 14px;
  color: rgba(10, 10, 10, 0.22);
  font-style: normal;
  font-weight: 800;
  font-size: 11px;
}

:global(.dark) .hero-signal i {
  color: rgba(255, 255, 255, 0.22);
}

.hero-signal span {
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
  font-size: 9px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero-signal strong {
  display: block;
  margin-top: 14px;
  color: var(--ink);
  font-size: 16px;
  font-weight: 800;
  line-height: 1.1;
}

.hero-signal p {
  margin-top: 7px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.35;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}

.hero-preview,
.b-card {
  padding: 10px;
  border-radius: var(--shell-radius);
  background: rgba(255, 255, 255, 0.45);
  box-shadow:
    inset 0 1px 0 var(--innerlight),
    0 0 0 1px var(--hairline),
    0 30px 60px -28px rgba(10, 10, 10, 0.22),
    0 12px 24px -16px rgba(10, 10, 10, 0.1);
}

.hero-preview {
  position: relative;
  transform: translateY(6px);
}

.hero-preview::before {
  content: '';
  position: absolute;
  inset: -16px;
  z-index: -1;
  border-radius: calc(var(--shell-radius) + 18px);
  background:
    linear-gradient(135deg, rgba(31, 77, 63, 0.2), transparent 36%),
    linear-gradient(315deg, rgba(201, 183, 154, 0.28), transparent 42%),
    repeating-linear-gradient(
      135deg,
      rgba(31, 77, 63, 0.08) 0,
      rgba(31, 77, 63, 0.08) 1px,
      transparent 1px,
      transparent 16px
    );
  filter: blur(4px);
  opacity: 0.78;
}

:global(.dark) .hero-preview,
:global(.dark) .b-card {
  background: rgba(255, 255, 255, 0.06);
}

.hero-preview .core,
.b-card .core {
  height: 100%;
  border-radius: var(--core-radius);
  background: linear-gradient(180deg, #fafaf7 0%, #efefeb 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.85),
    inset 0 0 0 1px var(--hairline);
  overflow: hidden;
}

:global(.dark) .hero-preview .core,
:global(.dark) .b-card .core {
  background: linear-gradient(180deg, #1a1a1a 0%, #0f0f0f 100%);
}

.hero-preview .core {
  position: relative;
  padding: 18px;
}

.runtime-preview .core::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(10, 10, 10, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(10, 10, 10, 0.035) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: linear-gradient(to bottom, black 0%, transparent 82%);
}

:global(.dark) .runtime-preview .core::before {
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.055) 1px, transparent 1px);
}

.runtime-topbar,
.runtime-head,
.runtime-stage-map,
.runtime-body-grid,
.runtime-trace {
  position: relative;
  z-index: 1;
}

.runtime-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.runtime-window {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 12px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
}

.runtime-window b {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-chip {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  background: rgba(31, 77, 63, 0.1);
  color: var(--accent);
  padding: 7px 10px;
  font-size: 11px;
  font-weight: 800;
}

.runtime-head {
  display: grid;
  min-height: 162px;
  grid-template-columns: minmax(0, 1fr) 118px;
  gap: 18px;
  align-items: center;
  border-radius: 1.45rem;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(255, 255, 255, 0.52)),
    linear-gradient(135deg, rgba(31, 77, 63, 0.08), transparent 50%);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 22px 46px -34px rgba(10, 10, 10, 0.25);
  padding: 22px;
}

:global(.dark) .runtime-head {
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.075), rgba(255, 255, 255, 0.04)),
    linear-gradient(135deg, rgba(143, 207, 183, 0.11), transparent 50%);
}

.runtime-head h3 {
  max-width: 15ch;
  margin: 0;
  color: var(--ink);
  font-size: clamp(24px, 2.7vw, 34px);
  font-weight: 800;
  line-height: 1.02;
  letter-spacing: 0;
}

.runtime-head p {
  max-width: 34ch;
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.55;
}

.runtime-radial {
  display: grid;
  width: 112px;
  height: 112px;
  place-items: center;
  border-radius: 999px;
  background:
    radial-gradient(circle at center, var(--canvas) 0 50%, transparent 51%),
    conic-gradient(var(--accent) 0 82%, rgba(10, 10, 10, 0.08) 82% 100%);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    0 18px 34px -28px rgba(10, 10, 10, 0.3);
  text-align: center;
}

.runtime-radial strong,
.runtime-radial span {
  grid-area: 1 / 1;
}

.runtime-radial strong {
  margin-top: -8px;
  color: var(--ink);
  font-size: 24px;
  font-weight: 900;
}

.runtime-radial span {
  max-width: 8ch;
  margin-top: 34px;
  color: var(--muted);
  font-size: 9px;
  font-weight: 800;
  line-height: 1.15;
  text-transform: uppercase;
}

.runtime-stage-map {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
  margin-top: 10px;
}

.runtime-stage {
  min-height: 64px;
  border-radius: 1rem;
  background: rgba(255, 255, 255, 0.55);
  box-shadow: inset 0 0 0 1px var(--hairline);
  padding: 10px;
}

:global(.dark) .runtime-stage {
  background: rgba(255, 255, 255, 0.055);
}

.runtime-stage span {
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
  font-size: 9px;
}

.runtime-stage strong {
  display: block;
  margin-top: 10px;
  color: var(--ink);
  font-size: 13px;
  font-weight: 900;
  line-height: 1.1;
}

.runtime-stage.active {
  background: var(--ink);
}

.runtime-stage.active span,
.runtime-stage.active strong {
  color: var(--canvas);
}

.runtime-body-grid {
  display: grid;
  grid-template-columns: 1.06fr 0.94fr;
  gap: 10px;
  margin-top: 10px;
}

.runtime-card {
  min-height: 114px;
  border-radius: 1.15rem;
  background: rgba(255, 255, 255, 0.68);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.72);
  color: var(--ink);
  padding: 15px;
  text-decoration: none;
}

:global(.dark) .runtime-card {
  background: rgba(255, 255, 255, 0.06);
}

.runtime-card strong {
  display: block;
  margin-top: 10px;
  color: var(--ink);
  font-size: 21px;
  font-weight: 900;
  line-height: 1;
}

.runtime-card p {
  margin: 10px 0 0;
  color: var(--muted);
  font-size: 11.5px;
  line-height: 1.45;
}

.runtime-card-dark {
  grid-row: span 2;
  background: linear-gradient(180deg, #1a1a1a 0%, #0f0f0f 100%);
}

.runtime-card-dark .meta,
.runtime-card-dark strong,
.runtime-card-dark p {
  color: #f2f2f0;
}

.runtime-card-dark p {
  color: rgba(242, 242, 240, 0.62);
}

.runtime-card-link {
  transition:
    transform 300ms var(--ease),
    background 300ms var(--ease);
}

.runtime-card-link:hover {
  transform: translateY(-2px);
  background: rgba(255, 255, 255, 0.86);
}

.runtime-meter {
  height: 8px;
  overflow: hidden;
  margin-top: 14px;
  border-radius: 999px;
  background: rgba(10, 10, 10, 0.08);
}

.runtime-meter span {
  display: block;
  width: 72%;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--accent), var(--sand));
}

.runtime-trace {
  margin-top: 8px;
  border-radius: 1.15rem;
  background: rgba(255, 255, 255, 0.58);
  box-shadow: inset 0 0 0 1px var(--hairline);
  padding: 13px;
}

:global(.dark) .runtime-trace {
  background: rgba(255, 255, 255, 0.055);
}

.runtime-trace p {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  margin: 0;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.5;
}

.runtime-trace p span,
.runtime-trace p b,
.runtime-trace p code {
  display: inline-flex;
  border-radius: 999px;
  background: rgba(31, 77, 63, 0.08);
  color: var(--muted);
  padding: 5px 8px;
  font-weight: 700;
}

.runtime-trace p code,
.runtime-trace p b {
  color: var(--accent);
}

.preview-bar {
  display: flex;
  gap: 6px;
  margin-bottom: 0;
}

.preview-bar span {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: rgba(10, 10, 10, 0.1);
}

:global(.dark) .preview-bar span {
  background: rgba(255, 255, 255, 0.18);
}

.preview-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.pv-cell {
  min-height: 126px;
  padding: 14px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.8);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.preview-flow-cell {
  min-height: auto;
}

.preview-flow {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.preview-flow span {
  display: inline-flex;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  padding: 6px 9px;
  font-size: 11px;
  font-weight: 800;
  line-height: 1;
}

:global(.dark) .pv-cell {
  background: rgba(255, 255, 255, 0.06);
}

.pv-cell.span2 {
  grid-column: span 2;
}

.pv-cell h4 {
  margin: 0 0 6px;
  color: var(--ink);
  font-size: 14px;
  font-weight: 700;
}

.pv-cell p {
  margin: 0;
  color: var(--muted);
  font-size: 11.5px;
  line-height: 1.6;
}

.pv-cell code {
  color: var(--accent);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
}

.pv-cell .meta,
.b-card .meta {
  margin-bottom: 10px;
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
  font-size: 10px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.num {
  margin-top: 12px;
  color: var(--ink);
  font-size: 34px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1;
}

.num small {
  margin-left: 4px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.between {
  justify-content: space-between;
}

.status-line {
  margin-top: 4px;
}

.pulse {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--accent);
  box-shadow: 0 0 0 0 rgba(31, 77, 63, 0.4);
  animation: pulse 2.4s var(--ease) infinite;
}

@keyframes pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(31, 77, 63, 0.4);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(31, 77, 63, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(31, 77, 63, 0);
  }
}

.ring {
  width: 38px;
  height: 38px;
  border-radius: 999px;
  background: conic-gradient(var(--accent) 0 67%, rgba(10, 10, 10, 0.08) 67% 100%);
  mask: radial-gradient(circle 14px, transparent 12px, black 13px);
  -webkit-mask: radial-gradient(circle 14px, transparent 12px, black 13px);
}

.skills-section {
  padding: 28px 0 48px;
}

.skill-radar {
  display: grid;
  grid-template-columns: 0.95fr 1.05fr;
  gap: 48px;
  align-items: end;
}

.skill-radar-copy h2 {
  max-width: 13ch;
  margin: 16px 0 0;
  color: var(--ink);
  font-size: clamp(38px, 5vw, 64px);
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0;
}

.skill-radar-copy p {
  max-width: 52ch;
  margin: 22px 0 0;
  color: var(--muted);
  font-size: 16px;
  line-height: 1.65;
}

.skill-radar-board {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.signal-card {
  display: flex;
  min-height: 142px;
  flex-direction: column;
  justify-content: space-between;
  border-radius: 1.25rem;
  background: rgba(255, 255, 255, 0.62);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.7),
    0 24px 44px -34px rgba(10, 10, 10, 0.2);
  color: var(--ink);
  padding: 18px;
  text-decoration: none;
}

:global(.dark) .signal-card {
  background: rgba(255, 255, 255, 0.06);
}

.signal-card span {
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
  font-size: 10px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.signal-card strong {
  display: block;
  margin-top: 14px;
  color: var(--ink);
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1;
}

.signal-card p {
  margin: 14px 0 0;
  color: var(--muted);
  font-size: 12.5px;
  line-height: 1.55;
}

.signal-card-dark {
  background: linear-gradient(180deg, #1a1a1a 0%, #0f0f0f 100%);
  color: #ececea;
}

.signal-card-dark span,
.signal-card-dark p {
  color: rgba(255, 255, 255, 0.58);
}

.signal-card-dark strong {
  color: #ffffff;
}

.section-head {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
  align-items: end;
  margin-bottom: 56px;
}

.section-head.compact {
  margin-bottom: 24px;
}

.section-head h2 {
  max-width: 14ch;
  margin: 16px 0 0;
  color: var(--ink);
  font-size: clamp(38px, 5vw, 64px);
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0;
}

.section-head p {
  max-width: 46ch;
  margin: 0;
  color: var(--muted);
  font-size: 16.5px;
  line-height: 1.55;
}

.marquee {
  overflow: hidden;
  padding: 40px 0 56px;
  mask-image: linear-gradient(90deg, transparent, black 12%, black 88%, transparent);
  -webkit-mask-image: linear-gradient(90deg, transparent, black 12%, black 88%, transparent);
}

.marquee-track {
  display: flex;
  width: max-content;
  gap: 44px;
  animation: track 80s linear infinite;
  will-change: transform;
}

.marquee:hover .marquee-track {
  animation-play-state: paused;
}

@keyframes track {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-50%);
  }
}

.marquee-item {
  display: inline-flex;
  align-items: center;
  color: var(--ink);
  font-size: clamp(18px, 2.1vw, 28px);
  font-weight: 700;
  letter-spacing: 0;
  opacity: 0.52;
  text-decoration: none;
  white-space: nowrap;
  transition:
    opacity 220ms var(--ease),
    color 220ms var(--ease);
}

.marquee-item:hover {
  color: var(--accent);
  opacity: 0.95;
}

.marquee-item em {
  margin-left: 44px;
  color: var(--muted);
  font-style: normal;
}

.block {
  position: relative;
  padding: 112px 0;
}

.bento {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 14px;
}

.b-card {
  transition:
    transform 600ms var(--ease),
    box-shadow 600ms var(--ease);
}

.b-card:hover {
  transform: translateY(-3px);
  box-shadow:
    inset 0 1px 0 var(--innerlight),
    0 0 0 1px var(--hairline),
    0 30px 70px -22px rgba(10, 10, 10, 0.22);
}

.b-card .core {
  display: flex;
  flex-direction: column;
  padding: 32px;
}

.b-card.dark .core {
  background: linear-gradient(180deg, #1a1a1a 0%, #0f0f0f 100%);
  color: #ececea;
}

.b-card h3 {
  margin: 0 0 10px;
  color: var(--ink);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: 0;
}

.b-card.dark h3 {
  color: #ffffff;
}

.b-card p {
  max-width: 39ch;
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.55;
}

.b-card.dark p,
.b-card.dark .meta {
  color: rgba(255, 255, 255, 0.6);
}

.span-2 {
  grid-column: span 2;
}

.span-3 {
  grid-column: span 3;
}

.row-2 {
  grid-row: span 2;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 18px;
}

.tag-row span {
  display: inline-flex;
  border-radius: 999px;
  background: rgba(31, 77, 63, 0.1);
  color: var(--accent);
  padding: 7px 10px;
  font-size: 12px;
  font-weight: 700;
}

.mission-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 24px;
}

.mission-metric {
  min-height: 112px;
  border-radius: 1rem;
  background: rgba(31, 77, 63, 0.06);
  box-shadow:
    inset 0 0 0 1px var(--hairline),
    inset 0 1px 0 rgba(255, 255, 255, 0.45);
  padding: 14px;
}

:global(.dark) .mission-metric {
  background: rgba(255, 255, 255, 0.06);
}

.mission-metric span {
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
  font-size: 9.5px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.mission-metric strong {
  display: block;
  margin-top: 12px;
  color: var(--ink);
  font-size: 20px;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1;
}

.mission-metric p {
  margin-top: 10px;
  font-size: 12px;
  line-height: 1.45;
}

.closing {
  position: relative;
  overflow: hidden;
  margin: 56px 28px 28px;
  padding: 112px 0;
  border-radius: 40px;
  background: linear-gradient(180deg, #131313 0%, #050505 100%);
  color: #f2f2f0;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    0 30px 80px -40px rgba(10, 10, 10, 0.6);
}

.closing::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(700px 360px at 30% 0%, rgba(90, 140, 122, 0.32), transparent 70%);
}

.closing-inner {
  position: relative;
  max-width: 1040px;
  margin: 0 auto;
  padding: 0 32px;
  text-align: center;
}

.closing h2 {
  max-width: 18ch;
  margin: 0 auto 18px;
  color: #f2f2f0;
  font-size: clamp(40px, 5.5vw, 76px);
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0;
}

.closing p {
  max-width: 48ch;
  margin: 0 auto 32px;
  color: rgba(242, 242, 240, 0.65);
  font-size: 17px;
  line-height: 1.55;
}

.closing .pill {
  background: var(--canvas);
  color: var(--ink);
}

.closing .pill .icon-wrap {
  background: rgba(10, 10, 10, 0.08);
}

.closing-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
}

.ghost.invert {
  background: rgba(255, 255, 255, 0.08);
  color: #f2f2f0;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.14);
}

.ghost.invert:hover {
  background: rgba(255, 255, 255, 0.16);
}

footer {
  padding: 36px 28px 28px;
  color: var(--muted);
  font-family:
    'JetBrains Mono',
    ui-monospace,
    monospace;
  font-size: 11px;
  letter-spacing: 0.06em;
}

.footer-row {
  display: flex;
  max-width: 1240px;
  justify-content: space-between;
  gap: 16px;
  margin: 0 auto;
  padding: 0 28px;
  flex-wrap: wrap;
}

footer a {
  color: var(--muted);
  text-decoration: none;
}

footer a:hover {
  color: var(--ink);
}

@media (max-width: 980px) {
  .hero {
    grid-template-columns: 1fr;
    min-height: auto;
    padding-top: 64px;
    padding-bottom: 80px;
  }

  .nav ul {
    display: none;
  }

  .hero-title {
    max-width: 100%;
  }

  .hero-title-main {
    white-space: normal;
  }

  .runtime-preview {
    max-width: 640px;
    margin: 0 auto;
  }

  .section-head {
    grid-template-columns: 1fr;
    gap: 18px;
  }

  .skill-radar {
    grid-template-columns: 1fr;
    gap: 24px;
  }

  .bento {
    grid-template-columns: 1fr;
  }

  .span-2,
  .span-3 {
    grid-column: span 1;
  }

  .row-2 {
    grid-row: auto;
  }

  .closing {
    margin: 28px 16px;
    padding: 80px 0;
  }
}

@media (max-width: 720px) {
  .wrap {
    padding: 0 18px;
  }

  .nav-shell {
    max-width: calc(100% - 24px);
  }

  .nav {
    gap: 10px;
    padding-left: 12px;
  }

  .brand span:last-child {
    max-width: 8rem;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .icon-btn {
    display: none;
  }

  .hero h1 {
    font-size: clamp(42px, 16vw, 64px);
  }

  .research-field-roller {
    width: 100%;
  }

  .research-field-item {
    font-size: 0.78em;
  }

  .hero-signal-row {
    grid-template-columns: 1fr;
  }

  .hero-signal {
    min-height: auto;
    box-shadow: inset 0 -1px 0 var(--hairline);
  }

  .hero-signal:last-child {
    box-shadow: none;
  }

  .runtime-head {
    grid-template-columns: 1fr;
    min-height: auto;
  }

  .runtime-radial {
    display: none;
  }

  .runtime-stage-map {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .runtime-body-grid {
    grid-template-columns: 1fr;
  }

  .runtime-card-dark {
    grid-row: auto;
  }

  .hero-preview .core,
  .b-card .core {
    padding: 22px;
  }

  .preview-grid {
    grid-template-columns: 1fr;
  }

  .skill-radar-board {
    grid-template-columns: 1fr;
  }

  .mission-metrics {
    grid-template-columns: 1fr;
  }

  .pv-cell.span2 {
    grid-column: auto;
  }

  .marquee-track {
    gap: 28px;
  }

  .marquee-item em {
    margin-left: 28px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .mesh::before,
  .mesh::after,
  .pulse,
  .research-field-track,
  .marquee-track {
    animation: none !important;
  }
}
</style>
