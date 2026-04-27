<template>
  <div class="landing">
    <!-- 顶部导航 -->
    <header class="navbar">
      <div class="nav-inner">
        <div class="logo">⚡ TransitAI</div>
        <nav class="nav-links">
          <a href="#pricing">价格</a>
          <a href="#compare">对比</a>
          <a href="#faq">FAQ</a>
        </nav>
        <div class="nav-actions">
          <router-link to="/login" class="btn-ghost">登录</router-link>
          <router-link to="/register" class="btn-primary">免费注册</router-link>
        </div>
      </div>
    </header>

    <!-- Hero -->
    <section class="hero">
      <div class="hero-inner">
        <div class="badge">🇨🇳 中国主体 · 合规运营 · 即开即用</div>
        <h1 class="hero-title">
          稳定合规的 <span class="hl">Claude API</span> 通道
        </h1>
        <p class="hero-subtitle">
          为开发者和企业 7×24 提供完整的<strong>服务、发票与对账</strong>能力。<br/>
          告别灰产中转的不稳定与跑路风险。
        </p>
        <div class="hero-cta">
          <router-link to="/register" class="btn-cta-primary">立即免费试用 →</router-link>
          <a href="#pricing" class="btn-cta-ghost">查看价格</a>
        </div>
        <div class="hero-trust">
          <span>✅ 工商注册主体</span>
          <span>✅ 可开发票</span>
          <span>✅ 多上游故障转移</span>
          <span>✅ 完整对账</span>
        </div>
      </div>
    </section>

    <!-- 核心优势 -->
    <section class="features">
      <h2 class="section-title">为什么选择 TransitAI</h2>
      <p class="section-sub">解决用 Claude API 真正会遇到的问题</p>
      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">🛡️</div>
          <h3>合规主体</h3>
          <p>中国大陆工商注册主体，可开普票与增值税专票，企业财务可正常入账。</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">⚡</div>
          <h3>多上游故障转移</h3>
          <p>单通道宕机 0.1 秒内自动切换备用通道。Telegram 实时告警。永远不让你的业务因 API 中断而停摆。</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">📊</div>
          <h3>项目级对账</h3>
          <p>每个 API Key 可绑定项目名 + 月度预算。CSV 导出含费用、模型、IP、扣费后余额。财务做账无忧。</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">🎯</div>
          <h3>原生官方模型</h3>
          <p>直连 Anthropic 官方 API，使用真实模型版本（Claude Opus 4.7 / 4.6 / Sonnet 4.5）。不反代、不拼车、Token 不掺水。</p>
        </div>
      </div>
    </section>

    <!-- 价格表 -->
    <section id="pricing" class="pricing">
      <h2 class="section-title">透明价格</h2>
      <p class="section-sub">按使用量付费，没有隐藏费用</p>

      <div class="pricing-grid">
        <!-- 入门版 -->
        <div class="price-card">
          <div class="price-header">
            <h3>入门版</h3>
            <p class="price-tagline">个人开发者起步</p>
          </div>
          <div class="price-amount">
            <span class="price-num">免费</span>
            <span class="price-unit">充多少到账多少</span>
          </div>
          <ul class="price-features">
            <li>✓ 全部 Claude 4.x 模型</li>
            <li>✓ 充值即用，无月费</li>
            <li>✓ Playground 在线调试</li>
            <li>✓ 基础对账明细</li>
            <li>✓ 1 个 API Key</li>
            <li class="muted">✗ 不开发票</li>
          </ul>
          <router-link to="/register" class="price-cta">免费注册</router-link>
        </div>

        <!-- 专业版（推荐）-->
        <div class="price-card featured">
          <div class="featured-badge">推荐</div>
          <div class="price-header">
            <h3>专业版</h3>
            <p class="price-tagline">中小团队 / SaaS 项目</p>
          </div>
          <div class="price-amount">
            <span class="price-num">¥99</span>
            <span class="price-unit">/月</span>
          </div>
          <ul class="price-features">
            <li>✓ 充 ¥99 <strong>实际到账 ¥120</strong></li>
            <li>✓ 5 个独立项目 API Key</li>
            <li>✓ RPM 60 / TPM 100k</li>
            <li>✓ <strong>普票 / 增值税专票</strong></li>
            <li>✓ 项目维度 CSV 导出</li>
            <li>✓ Telegram 预算告警</li>
            <li>✓ 邮件工单支持</li>
          </ul>
          <router-link to="/register" class="price-cta primary">立即开通</router-link>
        </div>

        <!-- 企业版 -->
        <div class="price-card">
          <div class="price-header">
            <h3>企业版</h3>
            <p class="price-tagline">上下游集成 / 大客户</p>
          </div>
          <div class="price-amount">
            <span class="price-num">¥499</span>
            <span class="price-unit">/月起</span>
          </div>
          <ul class="price-features">
            <li>✓ 充 ¥499 <strong>实际到账 ¥600</strong></li>
            <li>✓ 无限项目 API Key</li>
            <li>✓ RPM 600 / TPM 1M</li>
            <li>✓ <strong>季度 / 年度合同</strong>更优惠</li>
            <li>✓ 专属客服 + 技术对接</li>
            <li>✓ SLA 99.5% 保障</li>
            <li>✓ 私有化部署可议</li>
          </ul>
          <a href="mailto:sixweisix@gmail.com" class="price-cta">联系销售</a>
        </div>
      </div>

      <p class="pricing-note">
        💡 所有套餐共用统一计费规则：Opus 4.x ¥54/M 输入 ¥270/M 输出 · Sonnet 4.5 ¥32.4/M 输入 ¥162/M 输出 · 1.5x 倍率（vs 官方价）
      </p>
    </section>

    <!-- 对比表 -->
    <section id="compare" class="compare">
      <h2 class="section-title">vs 灰产中转站</h2>
      <p class="section-sub">为什么 B 端业务不能用便宜的灰产通道</p>

      <div class="compare-table-wrap">
        <table class="compare-table">
          <thead>
            <tr>
              <th></th>
              <th class="us">TransitAI</th>
              <th>灰产中转站</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td class="row-label">主体合规</td>
              <td class="us">✅ 工商注册</td>
              <td class="them">❌ 个人 / 离岸壳公司</td>
            </tr>
            <tr>
              <td class="row-label">开发票</td>
              <td class="us">✅ 普票 / 增值税专票</td>
              <td class="them">❌ 不开</td>
            </tr>
            <tr>
              <td class="row-label">资金安全</td>
              <td class="us">✅ 余额实时同步 + DB 审计</td>
              <td class="them">❓ 后台任意调整</td>
            </tr>
            <tr>
              <td class="row-label">对账明细</td>
              <td class="us">✅ CSV / 项目筛选 / 月度</td>
              <td class="them">❌ 只看到余额</td>
            </tr>
            <tr>
              <td class="row-label">模型来源</td>
              <td class="us">✅ Anthropic 官方原生</td>
              <td class="them">⚠️ 反代 Pro 套餐 / 拼车号</td>
            </tr>
            <tr>
              <td class="row-label">Token 计费</td>
              <td class="us">✅ 按官方规则准确计费</td>
              <td class="them">⚠️ 可能掺水（多扣 30-50%）</td>
            </tr>
            <tr>
              <td class="row-label">故障转移</td>
              <td class="us">✅ 多上游自动切换</td>
              <td class="them">❌ 单点宕机就罢工</td>
            </tr>
            <tr>
              <td class="row-label">运营连续性</td>
              <td class="us">✅ 公开主体长期运营</td>
              <td class="them">❓ 跑路风险</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- FAQ -->
    <section id="faq" class="faq">
      <h2 class="section-title">常见问题</h2>
      <div class="faq-list">
        <details class="faq-item">
          <summary>为什么不直接接官方 API？</summary>
          <p>Anthropic 官方 API 不对中国大陆开放支付，且需要美国信用卡。TransitAI 提供国内主体支付（支付宝 / 对公转账）和发票，让中国企业能合规使用。</p>
        </details>
        <details class="faq-item">
          <summary>价格比灰产同行贵？</summary>
          <p>是的，我们的倍率是 1.5x（官方价 × 1.5）。灰产通常 0.3-0.5x，因为他们用反代 Pro 账号 / 拼车号 / 模型替换 / Token 掺水来压低成本。一旦被 Anthropic 封号或他们跑路，你的数据和余额都会丢。我们的价值是<strong>稳定、合规、可对账</strong>，不是最便宜。</p>
        </details>
        <details class="faq-item">
          <summary>如何开发票？</summary>
          <p>专业版 / 企业版用户可直接在用户中心申请普票或增值税专票（电子发票，发送至预留邮箱）。开票主体为「引途信息咨询工作室」。</p>
        </details>
        <details class="faq-item">
          <summary>支持哪些模型？</summary>
          <p>当前上线：Claude Opus 4.7 / Claude Opus 4.6 / Claude Sonnet 4.5。后续将根据 Anthropic 发布节奏同步上线新模型。</p>
        </details>
        <details class="faq-item">
          <summary>SDK 兼容吗？</summary>
          <p>完全兼容 OpenAI SDK 格式（即 <code>chat/completions</code> 接口）。原有代码只需修改 base_url 和 api_key 两个参数即可切换过来。</p>
        </details>
      </div>
    </section>

    <!-- 底部 CTA -->
    <section class="cta-bottom">
      <h2>准备开始？</h2>
      <p>注册即送试用余额，无需信用卡</p>
      <router-link to="/register" class="btn-cta-primary">立即免费注册 →</router-link>
    </section>

    <!-- Footer -->
    <footer class="footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <div class="logo">⚡ TransitAI</div>
          <p>引途信息咨询工作室</p>
          <p class="footer-meta">合规 · 稳定 · 可对账</p>
        </div>
        <div class="footer-col">
          <h4>产品</h4>
          <a href="#pricing">价格</a>
          <a href="#compare">对比</a>
          <router-link to="/login">登录</router-link>
          <router-link to="/register">注册</router-link>
        </div>
        <div class="footer-col">
          <h4>条款</h4>
          <router-link to="/terms">用户协议</router-link>
          <router-link to="/privacy">隐私政策</router-link>
        </div>
        <div class="footer-col">
          <h4>联系</h4>
          <a href="mailto:sixweisix@gmail.com">sixweisix@gmail.com</a>
        </div>
      </div>
      <div class="footer-bottom">
        © 2026 TransitAI · 引途信息咨询工作室
      </div>
    </footer>
  </div>
</template>

<script setup>
// 纯静态页，无需脚本逻辑
</script>

<style scoped>
.landing { background: #fff; color: #1f2937; }

/* 导航 */
.navbar {
  position: sticky; top: 0; z-index: 50;
  background: rgba(255,255,255,0.92);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid #f1f5f9;
}
.nav-inner {
  max-width: 1200px; margin: 0 auto;
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 20px;
}
.logo { font-size: 18px; font-weight: 700; color: #6366f1; }
.nav-links { display: flex; gap: 24px; }
.nav-links a { color: #6b7280; font-size: 14px; text-decoration: none; }
.nav-links a:hover { color: #1f2937; }
.nav-actions { display: flex; gap: 10px; }
.btn-ghost {
  color: #6b7280; padding: 8px 16px; border-radius: 8px;
  text-decoration: none; font-size: 14px; font-weight: 500;
}
.btn-ghost:hover { background: #f3f4f6; }
.btn-primary {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; padding: 8px 18px; border-radius: 8px;
  text-decoration: none; font-size: 14px; font-weight: 600;
}

/* Hero */
.hero {
  background: linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%);
  padding: 80px 20px 100px;
}
.hero-inner { max-width: 880px; margin: 0 auto; text-align: center; }
.badge {
  display: inline-block; background: #fff; border: 1px solid #e0e7ff;
  color: #6366f1; padding: 6px 16px; border-radius: 20px;
  font-size: 13px; font-weight: 500; margin-bottom: 20px;
}
.hero-title {
  font-size: 44px; font-weight: 800; line-height: 1.2;
  margin: 0 0 20px; color: #111827;
}
.hl {
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text; background-clip: text; color: transparent;
}
.hero-subtitle {
  font-size: 17px; line-height: 1.7; color: #6b7280;
  margin: 0 0 32px;
}
.hero-subtitle strong { color: #1f2937; font-weight: 600; }
.hero-cta { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
.btn-cta-primary {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; padding: 14px 32px; border-radius: 10px;
  text-decoration: none; font-size: 16px; font-weight: 600;
  box-shadow: 0 6px 20px rgba(102,126,234,0.35);
}
.btn-cta-ghost {
  background: #fff; color: #6366f1; padding: 14px 28px;
  border-radius: 10px; text-decoration: none; font-size: 16px;
  font-weight: 600; border: 1px solid #e5e7eb;
}
.hero-trust {
  margin-top: 40px;
  display: flex; justify-content: center; gap: 24px;
  flex-wrap: wrap; color: #6b7280; font-size: 14px;
}

/* Section 通用 */
.section-title { text-align: center; font-size: 32px; font-weight: 700; margin: 0 0 8px; }
.section-sub { text-align: center; color: #6b7280; font-size: 15px; margin: 0 0 48px; }

/* Features */
.features { padding: 80px 20px; max-width: 1100px; margin: 0 auto; }
.features-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 20px;
}
.feature-card {
  background: #fff; border: 1px solid #f1f5f9;
  border-radius: 14px; padding: 24px;
}
.feature-icon { font-size: 32px; margin-bottom: 12px; }
.feature-card h3 { margin: 0 0 8px; font-size: 17px; }
.feature-card p { margin: 0; color: #6b7280; font-size: 14px; line-height: 1.6; }

/* Pricing */
.pricing { padding: 80px 20px; background: #fafbfc; }
.pricing-grid {
  max-width: 1100px; margin: 0 auto;
  display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 24px;
}
.price-card {
  background: #fff; border: 1px solid #e5e7eb;
  border-radius: 16px; padding: 32px 24px;
  position: relative; display: flex; flex-direction: column;
}
.price-card.featured {
  border: 2px solid #6366f1;
  box-shadow: 0 10px 30px rgba(102,126,234,0.15);
  transform: translateY(-8px);
}
.featured-badge {
  position: absolute; top: -12px; left: 50%; transform: translateX(-50%);
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; padding: 4px 14px; border-radius: 12px;
  font-size: 12px; font-weight: 600;
}
.price-header h3 { margin: 0 0 4px; font-size: 20px; }
.price-tagline { margin: 0 0 24px; color: #9ca3af; font-size: 13px; }
.price-amount { display: flex; align-items: baseline; gap: 6px; margin-bottom: 24px; }
.price-num { font-size: 38px; font-weight: 800; color: #111827; }
.price-unit { font-size: 14px; color: #9ca3af; }
.price-features { list-style: none; padding: 0; margin: 0 0 28px; flex: 1; }
.price-features li {
  padding: 8px 0; color: #4b5563; font-size: 14px;
  border-bottom: 1px solid #f9fafb;
}
.price-features li.muted { color: #d1d5db; }
.price-features strong { color: #6366f1; }
.price-cta {
  display: block; text-align: center; padding: 12px;
  border: 1px solid #e5e7eb; border-radius: 10px;
  text-decoration: none; color: #4b5563; font-weight: 600; font-size: 14px;
}
.price-cta.primary {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; border-color: transparent;
}
.pricing-note {
  text-align: center; max-width: 800px; margin: 32px auto 0;
  color: #6b7280; font-size: 13px; line-height: 1.6;
}

/* Compare */
.compare { padding: 80px 20px; max-width: 1000px; margin: 0 auto; }
.compare-table-wrap { overflow-x: auto; }
.compare-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.compare-table th, .compare-table td {
  padding: 14px 16px; text-align: left; font-size: 14px;
  border-bottom: 1px solid #f3f4f6;
}
.compare-table thead th { background: #f9fafb; font-weight: 600; }
.compare-table th.us, .compare-table td.us {
  background: linear-gradient(135deg, rgba(102,126,234,0.08), rgba(118,75,162,0.08));
  color: #4338ca; font-weight: 500;
}
.row-label { font-weight: 600; color: #1f2937; }
.them { color: #9ca3af; }

/* FAQ */
.faq { padding: 80px 20px; max-width: 800px; margin: 0 auto; }
.faq-list { display: flex; flex-direction: column; gap: 12px; }
.faq-item {
  background: #fff; border: 1px solid #e5e7eb;
  border-radius: 12px; padding: 20px 24px;
}
.faq-item summary {
  cursor: pointer; font-weight: 600; color: #1f2937;
  list-style: none; font-size: 15px;
}
.faq-item summary::-webkit-details-marker { display: none; }
.faq-item[open] summary { margin-bottom: 12px; }
.faq-item p { margin: 0; color: #6b7280; line-height: 1.7; font-size: 14px; }
.faq-item code {
  background: #f3f4f6; padding: 2px 6px; border-radius: 4px;
  font-family: 'SF Mono', Menlo, monospace; font-size: 13px;
}

/* CTA bottom */
.cta-bottom {
  padding: 80px 20px; text-align: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}
.cta-bottom h2 { font-size: 32px; margin: 0 0 8px; color: #fff; }
.cta-bottom p { margin: 0 0 28px; color: rgba(255,255,255,0.85); }
.cta-bottom .btn-cta-primary {
  background: #fff; color: #6366f1;
  box-shadow: 0 8px 24px rgba(0,0,0,0.15);
}

/* Footer */
.footer { background: #111827; color: #9ca3af; padding: 60px 20px 30px; }
.footer-inner {
  max-width: 1100px; margin: 0 auto;
  display: grid; grid-template-columns: 2fr 1fr 1fr 1fr; gap: 40px;
}
.footer-brand .logo { color: #a5b4fc; }
.footer-brand p { margin: 8px 0; font-size: 13px; }
.footer-meta { color: #6b7280; }
.footer-col h4 { color: #f9fafb; font-size: 14px; margin: 0 0 14px; }
.footer-col a {
  display: block; color: #9ca3af; font-size: 13px;
  text-decoration: none; padding: 4px 0;
}
.footer-col a:hover { color: #f9fafb; }
.footer-bottom {
  max-width: 1100px; margin: 40px auto 0;
  padding-top: 24px; border-top: 1px solid #1f2937;
  text-align: center; font-size: 12px; color: #6b7280;
}

/* Mobile */
@media (max-width: 720px) {
  .hero-title { font-size: 32px; }
  .nav-links { display: none; }
  .hero-trust { gap: 12px; font-size: 12px; }
  .footer-inner { grid-template-columns: 1fr; gap: 28px; }
  .price-card.featured { transform: none; }
  .compare-table th, .compare-table td { padding: 10px 8px; font-size: 12px; }
}
</style>
