module.exports = {
  themeConfig: {
    logo: '/nsbox.svg',
    nav: [
      {text: 'Home', link: '/'},
      {text: 'Guide', link: '/guide/'},
      {text: 'Images', link: '/images/'},
      {text: 'FAQ', link: '/faq/'},
      {text: 'Issues', link: 'https://ora.pm/project/211667/kanban'},
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
  description: 'A powerful pet container manager',
  head: [
    [
      'link', {
        rel: 'preconnect',
        href: 'https://fonts.googleapis.com/',
        crossorigin: ''
      }
    ],
    [
      'link', {
        rel: 'stylesheet',
        href:
            'https://fonts.googleapis.com/css2?family=Advent+Pro:wght@500;600&display=swap'
      }
    ],
  ],
  plugins: [['@vuepress/google-analytics', {'ga': 'UA-55018880-2'}]],
}
