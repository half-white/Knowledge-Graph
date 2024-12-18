// import './assets/main.css'

import { createApp } from 'vue'
import * as echats from 'echarts'
import Antd from 'ant-design-vue'
import 'ant-design-vue/dist/reset.css'
import App from './App1.vue'
//import App from './App.vue'

createApp(App).use(echats).use(Antd).mount('#app')
