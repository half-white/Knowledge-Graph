<template>
    <div id="app">
      <a-layout>
        <a-layout-header class="header">
          <div class="header-content">
            <h1>一键生成知识图谱</h1>
              <!-- <a href="http:" @click="toggleVisibility">历史数据</a> -->
          </div>
        </a-layout-header>
        <a-layout-content class="content" style="min-height: 100vh">
          <!-- 文本输入 -->
          <div>
            <label for="title">请输入文本:</label>
            <a-space>
              <a-input type="text" v-model:value="title" id="title" placeholder="输入文本" required/>
            </a-space>
            <!-- <input type="text" v-model="title" id="title" placeholder="输入文本" required> -->
            <a-button type="primary" @click="sendTitleToBackend" :disabled="!title">输入</a-button>
          </div>

          <!-- 上传文件 -->
          <div>
            <!-- <a-upload
              type="file"
              >
              <a-button @change="handleFileUpload">
                <upload-outlined></upload-outlined>
                选择文件
              </a-button>
            </a-upload> -->
            <label for="fileUpload">上传文件:</label>
            <input type="file" @change="handleFileUpload" id="fileUpload" required>
            <a-button type="primary" @click="submitFile" :disabled="!file">上传</a-button>
          </div>
        
          <!-- 错误信息 -->
          <div v-if="errorMessage">
            <p style="color: red;">{{ errorMessage }}</p>
          </div>

          <!-- 图谱展示 -->
          <div id="chart-container" style="display: flex; justify-content: center; align-items: center;">
            <div id="chart" style="width: 800px; height: 600px; border: 1px solid #000; background-color: white;">
            </div>
          </div>

          <!-- 历史数据 -->

        </a-layout-content>
        <a-layout-footer class="footer">
          <a href="http://172.16.20.10" target="_blank">
            智库咨询数字化平台
          </a>
        </a-layout-footer>
      </a-layout>
    </div>
  </template>
  
  <script setup>
import { ref, onMounted } from "vue";
import axios from "axios";
import * as echarts from "echarts";
import { message } from "ant-design-vue";

// 定义响应式数据
const title = ref("");
const file = ref(null);
const responseText = ref("");
const errorMessage = ref("");
const graphdata = ref("");
let myChart = null;

// 初始化图表
onMounted(() => {
  const chartDom = document.getElementById("chart");
  myChart = echarts.init(chartDom);
});

// 通过输入文本获取知识图谱
const sendTitleToBackend = async () => {
  try {
    const response = await axios.post("http://127.0.0.1:8080/api/model", { title: title.value });
    responseText.value = response.data.msg;
    graphdata.value = response.data.data;
    errorMessage.value = "";
    renderChart();
  } catch (error) {
    handleError(error);
  }
};

// 处理文件上传
const handleFileUpload = (event) => {
  file.value = event.target.files[0];
  // if (event.file.status === 'done') {
  //   message.success(`${event.file.name} file uploaded successfully`);
  // } else if (event.file.status === 'error') {
  //   message.error(`${event.file.name} file upload failed.`);
  // } 
};

// 通过上传文件获取知识图谱
const submitFile = async () => {
  if (!file.value) {
    errorMessage.value = "Please select a file to upload.";
    return;
  }

  const formData = new FormData();
  formData.append("file", file.value);

  try {
    const response = await axios.post("http://127.0.0.1:8080/api/upload", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    graphdata.value = response.data.data;
    file.value = null;
    errorMessage.value = "";
    renderChart();
  } catch (error) {
    handleError(error);
  }
};

// 获取历史数据列表

// 通过查询历史数据获取知识图谱


// 渲染图表
const renderChart = () => {
  if (!graphdata.value) return;

  const parsedData = JSON.parse(graphdata.value);
  const data = parsedData.nodes;
  const link = parsedData.links;
  const categories = parsedData.categories;

  const option = {
    title: { text: "知识图谱" },
    tooltip: {
      formatter: function (x) {
        return x.data.des;
      },
    },
    toolbox: {
      show: true,
      feature: {
        restore: { show: true },
        saveAsImage: { show: true },
      },
    },
    legend: [
      {
        data: categories.map((a) => a.name),
      },
    ],
    series: [
      {
        type: "graph",
        layout: "force",
        symbolSize: 30,
        roam: true,
        edgeSymbol: ["circle", "arrow"],
        edgeSymbolSize: [2, 10],
        force: {
          repulsion: 500,
          edgeLength: [20, 50],
          friction: 0.2,
          gravity: 0.3,
        },
        draggable: true,
        lineStyle: {
          normal: { width: 2, color: "#000000" },
        },
        edgeLabel: {
          normal: {
            show: true,
            formatter: function (x) {
              return x.data.value;
            },
            color:'#000',
            fontSize:12
          },
        },
        label: { normal: { show: true } },
        data: data,
        links: link,
        categories: categories,
      },
    ],
  };

  myChart.setOption(option);
};

// 错误处理函数
const handleError = (error) => {
  if (error.response) {
    errorMessage.value = `Error: ${error.response.status} ${error.response.statusText}`;
  } else if (error.request) {
    errorMessage.value = "Error: No response received from server.";
  } else {
    errorMessage.value = `Error: ${error.message}`;
  }
  console.error("Error:", error);
};
  </script>
  
  <style>
  #app {
    font-family: Avenir, Helvetica, Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
    text-align: center;
  }

  #app .header {
    background-color: gray;
    text-align: left;
    padding: 0 20px;
  }

  #app .header-content {
    display: flex;
    height: 100%;
    gap: 20px;
  }

  #app .footer {
    background-color: gray;
    text-align: center;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 1;
  }

  #app .content {
    padding: 20px;
    margin-bottom: 50px;
  }

  form {
    margin-bottom: 20px;
  }
  
  input {
    padding: 10px;
    font-size: 16px;
    margin-right: 10px;
  }
  
  button {
    padding: 10px 20px;
    font-size: 16px;
  }
  </style>