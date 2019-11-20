module.exports = {
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/' },
      { text: 'Images', link: '/images/' },
      { text: 'FAQ', link: '/faq/' },
      { text: 'Source', link: 'https://github.com/refi64/nsbox' },
    ],
    sidebar: 'auto',
    smoothScroll: true,
  },
  title: 'nsbox',
  plugins: [
    '@vuepress/google-analytics',
    {
      'ga': 'UA-55018880-2'
    }
  ],
}
