<template>
  <router-view />
</template>

<style>
/* iOS Safari 在 input/textarea/select font-size < 16px 时会自动放大,
   触发后页面状态被破坏导致顶栏看不到. 全局强制 16px 防放大. */
input,
textarea,
select,
.el-input__inner,
.el-textarea__inner,
.el-input-number__inner {
  font-size: 16px !important;
}
</style>
