<template>
  <div class="legal-page">
    <header class="legal-header">
      <button class="back-btn" @click="$router.back()">← 返回</button>
      <span class="legal-title">用户服务协议</span>
    </header>

    <div class="legal-content">
      <h2>TransitAI 用户服务协议</h2>
      <p class="legal-meta">最后更新：2026 年 4 月</p>

      <p>欢迎使用 TransitAI（以下简称"本平台"）。在使用本平台服务前，请您仔细阅读并充分理解本协议。一旦您完成注册或开始使用本平台服务，即视为您已阅读、理解并同意本协议的全部内容。</p>

      <h3>一、服务说明</h3>
      <p>本平台为 Anthropic Claude 等大语言模型提供 API 中转服务，接口格式兼容 OpenAI SDK，方便用户使用标准客户端调用模型。</p>

      <h3>二、账户注册与使用</h3>
      <p>1. 用户应使用真实有效的邮箱完成注册，并对账户密码的安全负责。</p>
      <p>2. 您不得将自己的账户、API Key 转让、出借给第三方使用。因您账户被他人盗用产生的损失，由您自行承担。</p>
      <p>3. 本平台有权对违规账户采取限制、冻结或注销等措施。</p>

      <h3>三、计费与价格透明</h3>
      <p>1. 所有模型按实际 token 用量计费，价格以人民币计价。</p>
      <p>2. 平台采用倍率定价，倍率明牌公开，用户可在"模型与价格"页面查看每个模型的实际单价。</p>
      <p>3. <strong>用户请求什么模型即调用什么模型，平台不会在后台进行模型替换。</strong></p>
      <p>4. 余额不足时请求将返回 402 Payment Required 错误。</p>

      <h3>四、充值与退款</h3>
      <p>1. 充值金额将立即计入账户余额，由支付宝等合规支付渠道处理。</p>
      <p>2. 余额一经充值，原则上不予退款。如确需退款，请联系客服邮箱说明情况。</p>
      <p>3. 长期未使用的账户余额，本平台保留依法进行处理的权利。</p>

      <h3>五、合规使用</h3>
      <p>1. 您不得使用本平台进行任何违反中国法律法规的活动，包括但不限于生成违法内容、网络攻击、欺诈等。</p>
      <p>2. 您应遵守上游服务商（Anthropic 等）的使用条款。本平台假设您具有合法的使用资质。</p>
      <p>3. 禁止将本平台 API 用于：自动化注册薅羊毛、刷单、规避内容审核、生成色情/暴恐/违法内容等。</p>
      <p>4. 一经发现违规，平台有权立即停止服务、不退还余额，并保留追究法律责任的权利。</p>

      <h3>六、风控措施</h3>
      <p>本平台对异常请求（高频刷量、盗刷信用卡、滥用等）将采取频率限制、异常告警等防护措施。如您的请求被风控系统拦截，请联系客服处理。</p>

      <h3>七、责任限制</h3>
      <p>1. 上游模型服务的可用性、响应内容质量、延迟等不在本平台完全可控范围内。</p>
      <p>2. 因不可抗力（自然灾害、网络故障、政策变化等）导致的服务中断，本平台不承担赔偿责任。</p>
      <p>3. 本平台对模型输出内容不做担保，用户应对模型输出的使用承担相应责任。</p>

      <h3>八、协议修改</h3>
      <p>本平台有权根据法律法规调整或服务变化修订本协议。修订后的协议将在网站公示，继续使用即视为接受新协议。</p>

      <h3>九、联系方式</h3>
      <p>如有疑问，可通过站内反馈或邮件 sixweisix@gmail.com 联系我们。</p>
    </div>
  </div>
</template>

<script setup></script>

<style scoped>
.legal-page {
  min-height: 100vh;
  background: #f5f7fa;
}
.legal-header {
  position: sticky;
  top: 0;
  background: #fff;
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
  border-bottom: 1px solid #f3f4f6;
  z-index: 10;
}
.back-btn {
  background: none;
  border: none;
  font-size: 14px;
  color: #667eea;
  cursor: pointer;
  font-weight: 600;
}
.legal-title {
  font-size: 16px;
  font-weight: 700;
  color: #1f2937;
}
.legal-content {
  max-width: 720px;
  margin: 0 auto;
  padding: 20px 18px 40px;
  background: #fff;
  font-size: 14px;
  line-height: 1.75;
  color: #374151;
}
.legal-content h2 {
  font-size: 20px;
  font-weight: 800;
  color: #1f2937;
  margin: 0 0 6px;
}
.legal-meta {
  color: #9ca3af;
  font-size: 12px;
  margin: 0 0 18px;
}
.legal-content h3 {
  font-size: 15px;
  font-weight: 700;
  color: #1f2937;
  margin: 22px 0 10px;
  padding-left: 8px;
  border-left: 3px solid #667eea;
}
.legal-content p {
  margin: 0 0 10px;
}
.legal-content strong {
  color: #ef4444;
}
</style>
