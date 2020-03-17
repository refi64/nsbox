module.exports = {
  themeConfig: {
    nav: [
      {text: 'Home', link: '/'},
      {text: 'Guide', link: '/guide/'},
      {text: 'Images', link: '/images/'},
      {text: 'FAQ', link: '/faq/'},
      {text: 'Source', link: 'https://github.com/refi64/nsbox'},
    ],
    sidebar: 'auto',
    smoothScroll: true,
    algolia: {
      apiKey: '38206f8f24dcc0d443c475ef4d13fac4',
      indexName: 'nsbox',
    },
  },
  title: 'nsbox',
  plugins: ['@vuepress/google-analytics', {'ga': 'UA-55018880-2'}],
}
