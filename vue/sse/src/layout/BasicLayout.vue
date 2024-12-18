<template>
    <div id="basicLayout">
        <a-layout>
            <a-layout-header>Header</a-layout-header>
            <a-layout-content>Content</a-layout-content>
            <a-layout-footer class="footer">
                <a href="http://172.16.20.10" target="_blank">
                    首页(智库数字化系统)
                </a>
            </a-layout-footer>
        </a-layout>
    </div>
</template>

<script setup>

</script>

<style scoped>
#basicLayout .footer {
    background-color: gray;
    text-align: center;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 1;
}
</style>