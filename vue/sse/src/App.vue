<template>
  <div id="app">
    <h1>大模型拆解文本</h1>
    <form @submit.prevent="sendTitleToBackend">
      <div>
        <label for="title">请输入文本:</label>
        <input type="text" v-model="title" id="title" required>
      </div>
      <button type="submit">输入</button>
    </form>
    <div v-if="responseText">
      <h2>文本拆解为：</h2>
      <p style="color: black;">{{ responseText }}</p>
    </div>
    <div v-if="errorMessage">
      <p style="color: red;">{{ errorMessage }}</p>
    </div>
  </div>
</template>

<script>
import axios from 'axios';

export default {
  name: 'App',
  data() {
    return {
      title: '',
      responseText: '',
      errorMessage: '',
    };
  },
  methods: {
    async sendTitleToBackend() {
      try {
        const response = await axios.post('http://127.0.0.1:8080/api/model', { title: this.title });
        this.responseText = response.data.msg; // 假设后端返回 { text: '...' }
        this.errorMessage = ''; // 清除任何先前的错误信息
      } catch (error) {
        if (error.response) {
          // 服务器响应了一个状态码，范围在2xx之外
          this.errorMessage = `Error: ${error.response.status} ${error.response.statusText}`;
        } else if (error.request) {
          // 请求已经发出，但没有收到响应
          this.errorMessage = 'Error: No response received from server.';
        } else {
          // 其他错误
          this.errorMessage = `Error: ${error.message}`;
        }
        console.error('Error sending title to backend:', error);
      }
    },
  }
};
</script>

<style>
#app {
  background-color: aliceblue;
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-align: center;
  color: black;
  margin-top: 60px;
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