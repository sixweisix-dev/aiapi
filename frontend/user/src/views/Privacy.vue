<template>
  <div class="legal-page">
    <header class="legal-header">
      <button class="back-btn" @click="$router.back()">← 返回</button>
      <span class="legal-title">隐私政策</span>
    </header>

    <div class="legal-content">
      <h2>TransitAI 隐私政策</h2>
      <p class="legal-meta">最后更新：2026 年 4 月</p>

      <p>本平台高度重视用户隐私。本政策说明我们如何收集、使用、存储和保护您的个人信息。</p>

      <h3>一、我们收集的信息</h3>
      <p>1. <strong>账户信息</strong>：邮箱、加密后的密码哈希。</p>
      <p>2. <strong>使用日志</strong>：API 请求时间、模型、token 用量、状态码、IP 地址、请求耗时等技术数据。</p>
      <p>3. <strong>支付信息</strong>：充值金额、订单号、支付方式、支付状态。我们不存储您的银行卡或支付密码。</p>

      <h3>二、关于对话内容</h3>
      <p>1. 您通过 API 提交的对话内容（messages）会被实时转发到上游模型服务商（如 Anthropic）。</p>
      <p>2. <strong>本平台严禁出售或共享用户对话内容给第三方用于模型训练。</strong></p>
      <p>3. 为故障排查目的，部分元数据（不含完整对话内容）会留存最长 90 天。</p>

      <h3>三、信息使用方式</h3>
      <p>1. 提供和维护服务（鉴权、计费、限流）。</p>
      <p>2. 改善服务体验、统计分析。</p>
      <p>3. 应法律要求或司法机关合法申请提供必要信息。</p>

      <h3>四、信息安全</h3>
      <p>1. 平台采用 HTTPS 加密传输、密码哈希存储、API Key 单向哈希等安全措施。</p>
      <p>2. 服务器部署在合规的云服务商上，数据库定期备份。</p>
      <p>3. 尽管我们采取合理措施保护您的信息，但请您理解互联网传输不存在 100% 安全。</p>

      <h3>五、Cookie 与本地存储</h3>
      <p>本平台使用 localStorage 保存登录令牌（JWT），用于维持会话状态。退出登录时令牌会被清除。</p>

      <h3>六、未成年人保护</h3>
      <p>本平台不面向 14 岁以下未成年人提供服务。如发现未成年人未经监护人同意注册，请联系我们删除账户。</p>

      <h3>七、您的权利</h3>
      <p>您有权随时：</p>
      <p>1. 查看自己的账户信息、消费明细。</p>
      <p>2. 修改密码、删除账户。</p>
      <p>3. 通过邮件联系我们要求导出或删除个人数据。</p>

      <h3>八、政策变更</h3>
      <p>如政策有重大变更，我们将通过站内通知或邮件告知您。继续使用即视为接受新政策。</p>

      <h3>九、联系我们</h3>
      <p>邮箱：sixweisix@gmail.com</p>
    </div>
  </div>
</template>

<script setup></script>

<style scoped>
.legal-page { min-height: 100vh; background: #f5f7fa; }
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
  background: none; border: none;
  font-size: 14px; color: #667eea;
  cursor: pointer; font-weight: 600;
}
.legal-title { font-size: 16px; font-weight: 700; color: #1f2937; }
.legal-content {
  max-width: 720px;
  margin: 0 auto;
  padding: 20px 18px 40px;
  background: #fff;
  font-size: 14px; line-height: 1.75; color: #374151;
}
.legal-content h2 { font-size: 20px; font-weight: 800; color: #1f2937; margin: 0 0 6px; }
.legal-meta { color: #9ca3af; font-size: 12px; margin: 0 0 18px; }
.legal-content h3 {
  font-size: 15px; font-weight: 700; color: #1f2937;
  margin: 22px 0 10px; padding-left: 8px;
  border-left: 3px solid #667eea;
}
.legal-content p { margin: 0 0 10px; }
.legal-content strong { color: #ef4444; }
</style>
