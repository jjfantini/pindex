import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/pindex/',
  title: 'pindex',
  description: 'Vectorless reasoning-RAG for PDFs — a single-binary Go CLI',
  cleanUrls: true,
  ignoreDeadLinks: false,
  srcExclude: ['README.md'],
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/getting-started' },
      { text: 'Features', link: '/features' },
      { text: 'Architecture', link: '/architecture' },
      { text: 'Guides', link: '/guides/index-a-pdf' },
      { text: 'Reference', link: '/reference/pindex' }
    ],
    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'What is pindex?', link: '/' },
          { text: 'Installation', link: '/installation' },
          { text: 'Getting Started', link: '/getting-started' },
          { text: 'How It Works', link: '/how-it-works' },
          { text: 'Effort Levels', link: '/effort-levels' },
          { text: 'Features', link: '/features' },
          { text: 'Architecture', link: '/architecture' }
        ]
      },
      {
        text: 'Guides',
        items: [
          { text: 'Index a PDF', link: '/guides/index-a-pdf' },
          { text: 'Ask Questions', link: '/guides/ask-questions' },
          { text: 'Batch Indexing', link: '/guides/batch-indexing' },
          { text: 'Choosing an Extractor', link: '/guides/choosing-an-extractor' },
          { text: 'Evaluate on FinanceBench', link: '/guides/evaluate-financebench' },
          { text: 'Configuration', link: '/guides/configuration' }
        ]
      },
      {
        text: 'CLI Reference',
        items: [
          { text: 'pindex', link: '/reference/pindex' },
          { text: 'pindex index', link: '/reference/pindex_index' },
          { text: 'pindex ask', link: '/reference/pindex_ask' },
          { text: 'pindex eval', link: '/reference/pindex_eval' },
          { text: 'pindex extract', link: '/reference/pindex_extract' }
        ]
      }
    ],
    search: {
      provider: 'local'
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/jjfantini/pindex' }
    ]
  }
})
