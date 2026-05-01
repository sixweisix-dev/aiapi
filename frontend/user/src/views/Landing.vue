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
        <div class="badge">🚀 稳定直连 · 极速响应 · 即开即用</div>
        <h1 class="hero-title">
          开发者首选的 <span class="hl">Claude API</span> 平台
        </h1>
        <p class="hero-subtitle">
          为个人开发者和企业团队提供稳定、透明、可对账的<strong> Claude API 接入服务</strong>。<br/>
          告别不稳定与跑路风险，专注构建你的产品。
        </p>
        <div class="hero-cta">
          <router-link to="/register" class="btn-cta-primary">立即免费试用 →</router-link>
          <a href="#pricing" class="btn-cta-ghost">查看价格</a>
        </div>
        <div class="hero-trust">
          <span>✅ 官方原生模型</span>
          <span>✅ 按量计费无月费</span>
          <span>✅ 多上游故障转移</span>
          <span>✅ 完整对账明细</span>
        </div>
      </div>
    </section>

    <!-- 核心优势 -->
    <section class="features">
      <h2 class="section-title">为什么选择 TransitAI</h2>
      <p class="section-sub">解决开发者使用 Claude API 真正会遇到的问题</p>
      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">🎯</div>
          <h3>原生官方模型</h3>
          <p>直连 Anthropic 官方 API，使用真实模型版本（Claude Opus 4.7 / 4.6 / Sonnet 4.5）。不反代、不拼车、Token 不掺水。</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">⚡</div>
          <h3>多上游故障转移</h3>
          <p>单通道宕机 0.1 秒内自动切换备用通道，永远不让你的业务因 API 中断而停摆。</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">📊</div>
          <h3>项目级对账</h3>
          <p>每个 API Key 可绑定项目名 + 月度预算。CSV 导出含费用、模型、IP、扣费后余额，财务做账无忧。</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">🔒</div>
          <h3>安全透明</h3>
          <p>余额实时同步 + 数据库全量审计，每一笔消费有据可查，资金安全有保障。</p>
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
            <li>✓ 兑换码直接充值</li>
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
            <li>✓ 项目维度 CSV 导出</li>
            <li>✓ 预算告警通知</li>
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
            <li>✓ 专属客服 + 技术对接</li>
            <li>✓ SLA 99.5% 保障</li>
            <li>✓ 私有化部署可议</li>
          </ul>
          <a href="mailto:sixweisix@gmail.com" class="price-cta">联系销售</a>
        </div>
      </div>

      <p class="pricing-note">
        💡 统一计费规则：Opus 4.x ¥54/M 输入 ¥270/M 输出 · Sonnet 4.5 ¥32.4/M 输入 ¥162/M 输出 · 1.5x 倍率
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
              <td class="row-label">资金安全</td>
              <td class="us">✅ 余额实时同步 + 审计日志</td>
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
              <td class="us">✅ 长期稳定运营</td>
              <td class="them">❓ 随时跑路风险</td>
            </tr>
            <tr>
              <td class="row-label">数据安全</td>
              <td class="us">✅ 独立账户隔离</td>
              <td class="them">❌ 共享账号数据混用</td>
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
          <summary>如何开始使用？</summary>
          <p>注册账号后，通过兑换码充值余额，即可在控制台创建 API Key 开始使用。全流程 5 分钟内完成，无需任何海外信用卡。</p>
        </details>
        <details class="faq-item">
          <summary>价格比灰产同行贵？</summary>
          <p>是的，我们的倍率是 1.5x（官方价 × 1.5）。灰产通常 0.3-0.5x，因为他们使用反代 Pro 账号或拼车号，Token 掺水严重。一旦被封号或跑路，你的余额将血本无归。我们的价值是<strong>稳定、透明、可对账</strong>，不是最便宜。</p>
        </details>
        <details class="faq-item">
          <summary>如何充值？</summary>
          <p>在闲鱼购买对应档位的兑换码，登录后进入「充值 & 会员」页面输入兑换码，余额立即到账。支持 ¥100 / ¥300 / ¥500 / ¥1000 / ¥3000 多个档位。</p>
        </details>
        <details class="faq-item">
          <summary>支持哪些模型？</summary>
          <p>当前上线：Claude Opus 4.7 / Claude Opus 4.6 / Claude Sonnet 4.5。后续将根据 Anthropic 发布节奏同步上线新模型。</p>
        </details>
        <details class="faq-item">
          <summary>SDK 兼容吗？</summary>
          <p>完全兼容 OpenAI SDK 格式（即 <code>chat/completions</code> 接口）。原有代码只需修改 base_url 和 api_key 两个参数即可切换。</p>
        </details>
        <details class="faq-item">
          <summary>会员和普通充值有什么区别？</summary>
          <p>普通充值按量计费，适合轻量使用。会员套餐额外解锁更高 RPM/TPM 速率限制和更多 API Key，适合有批量请求需求的团队。</p>
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
          <p class="footer-meta">稳定 · 透明 · 可对账</p>
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
        © 2026 TransitAI · All rights reserved.
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
  background: rgba(255,255,255,0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid #f1f5f9;
}
.nav-inner {
  max-width: 1200px; margin: 0 auto;
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 24px;
}
.logo { font-size: 18px; font-weight: 800; color: #6366f1; letter-spacing: -0.3px; }
.nav-links { display: flex; gap: 28px; }
.nav-links a { color: #6b7280; font-size: 14px; text-decoration: none; font-weight: 500; }
.nav-links a:hover { color: #1f2937; }
.nav-actions { display: flex; gap: 10px; }
.btn-ghost {
  color: #6b7280; padding: 8px 16px; border-radius: 8px;
  text-decoration: none; font-size: 14px; font-weight: 500;
}
.btn-ghost:hover { background: #f3f4f6; color: #1f2937; }
.btn-primary {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; padding: 8px 18px; border-radius: 8px;
  text-decoration: none; font-size: 14px; font-weight: 600;
  box-shadow: 0 2px 8px rgba(102,126,234,0.3);
}

/* Hero */
.hero {
  background: linear-gradient(135deg, #f8f7ff 0%, #eef2ff 50%, #faf5ff 100%);
  padding: 100px 24px 120px;
  position: relative; overflow: hidden;
}
.hero::before {
  content: '';
  position: absolute; top: -50%; left: -50%;
  width: 200%; height: 200%;
  background: radial-gradient(circle at 30% 40%, rgba(102,126,234,0.08) 0%, transparent 50%),
              radial-gradient(circle at 70% 60%, rgba(118,75,162,0.06) 0%, transparent 50%);
}
.hero-inner { max-width: 900px; margin: 0 auto; text-align: center; position: relative; }
.badge {
  display: inline-flex; align-items: center; gap: 6px;
  background: #fff; border: 1px solid #e0e7ff;
  color: #6366f1; padding: 8px 20px; border-radius: 24px;
  font-size: 13px; font-weight: 600; margin-bottom: 28px;
  box-shadow: 0 2px 8px rgba(99,102,241,0.1);
}
.hero-title {
  font-size: 52px; font-weight: 800; line-height: 1.15;
  margin: 0 0 24px; color: #111827; letter-spacing: -1px;
}
.hl {
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text; background-clip: text; color: transparent;
}
.hero-subtitle {
  font-size: 18px; line-height: 1.7; color: #6b7280;
  margin: 0 0 40px; max-width: 680px; margin-left: auto; margin-right: auto;
}
.hero-subtitle strong { color: #1f2937; font-weight: 600; }
.hero-cta { display: flex; gap: 14px; justify-content: center; flex-wrap: wrap; margin-bottom: 48px; }
.btn-cta-primary {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; padding: 16px 36px; border-radius: 12px;
  text-decoration: none; font-size: 16px; font-weight: 700;
  box-shadow: 0 8px 24px rgba(102,126,234,0.4);
  transition: transform 0.2s, box-shadow 0.2s;
}
.btn-cta-primary:hover { transform: translateY(-2px); box-shadow: 0 12px 32px rgba(102,126,234,0.45); }
.btn-cta-ghost {
  background: #fff; color: #6366f1; padding: 16px 32px;
  border-radius: 12px; text-decoration: none; font-size: 16px;
  font-weight: 600; border: 1.5px solid #e0e7ff;
  transition: border-color 0.2s;
}
.btn-cta-ghost:hover { border-color: #a5b4fc; }
.hero-trust {
  display: flex; justify-content: center; gap: 28px;
  flex-wrap: wrap; color: #6b7280; font-size: 14px; font-weight: 500;
}
.hero-trust span { display: flex; align-items: center; gap: 4px; }

/* Section 通用 */
.section-title { text-align: center; font-size: 34px; font-weight: 800; margin: 0 0 10px; letter-spacing: -0.5px; }
.section-sub { text-align: center; color: #6b7280; font-size: 16px; margin: 0 0 52px; }

/* Features */
.features { padding: 100px 24px; max-width: 1100px; margin: 0 auto; }
.features-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 24px;
}
.feature-card {
  background: linear-gradient(145deg, #fff, #fafbff);
  border: 1px solid #f1f5f9;
  border-radius: 18px; padding: 28px;
  transition: transform 0.2s, box-shadow 0.2s;
}
.feature-card:hover { transform: translateY(-4px); box-shadow: 0 12px 32px rgba(0,0,0,0.08); }
.feature-icon { font-size: 36px; margin-bottom: 14px; }
.feature-card h3 { margin: 0 0 10px; font-size: 18px; font-weight: 700; color: #111827; }
.feature-card p { margin: 0; color: #6b7280; font-size: 14px; line-height: 1.7; }

/* Pricing */
.pricing { padding: 100px 24px; background: linear-gradient(180deg, #fafbff, #f5f3ff); }
.pricing-grid {
  max-width: 1100px; margin: 0 auto;
  display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
}
.price-card {
  background: #fff; border: 1.5px solid #e5e7eb;
  border-radius: 20px; padding: 36px 28px;
  position: relative; display: flex; flex-direction: column;
  transition: transform 0.2s, box-shadow 0.2s;
}
.price-card:hover { transform: translateY(-4px); box-shadow: 0 16px 40px rgba(0,0,0,0.08); }
.price-card.featured {
  border: 2px solid #6366f1;
  box-shadow: 0 12px 36px rgba(102,126,234,0.18);
  transform: translateY(-10px);
}
.price-card.featured:hover { transform: translateY(-14px); }
.featured-badge {
  position: absolute; top: -14px; left: 50%; transform: translateX(-50%);
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; padding: 5px 18px; border-radius: 14px;
  font-size: 12px; font-weight: 700; letter-spacing: 0.3px;
}
.price-header h3 { margin: 0 0 4px; font-size: 22px; font-weight: 700; color: #111827; }
.price-tagline { margin: 0 0 28px; color: #9ca3af; font-size: 14px; }
.price-amount { display: flex; align-items: baseline; gap: 8px; margin-bottom: 28px; }
.price-num { font-size: 42px; font-weight: 800; color: #111827; letter-spacing: -1px; }
.price-unit { font-size: 14px; color: #9ca3af; }
.price-features { list-style: none; padding: 0; margin: 0 0 32px; flex: 1; }
.price-features li {
  padding: 10px 0; color: #4b5563; font-size: 14px;
  border-bottom: 1px solid #f9fafb; line-height: 1.5;
}
.price-features strong { color: #6366f1; font-weight: 700; }
.price-cta {
  display: block; text-align: center; padding: 14px;
  border: 1.5px solid #e5e7eb; border-radius: 12px;
  text-decoration: none; color: #4b5563; font-weight: 600; font-size: 15px;
  transition: all 0.2s;
}
.price-cta:hover { border-color: #6366f1; color: #6366f1; }
.price-cta.primary {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; border-color: transparent;
  box-shadow: 0 4px 16px rgba(102,126,234,0.35);
}
.price-cta.primary:hover { opacity: 0.92; transform: translateY(-1px); }
.pricing-note {
  text-align: center; max-width: 800px; margin: 40px auto 0;
  color: #9ca3af; font-size: 13px; line-height: 1.8;
  background: #fff; border-radius: 12px; padding: 16px 24px;
  border: 1px solid #f1f5f9;
}

/* Compare */
.compare { padding: 100px 24px; max-width: 1000px; margin: 0 auto; }
.compare-table-wrap { overflow-x: auto; border-radius: 16px; }
.compare-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 16px; overflow: hidden;
  box-shadow: 0 4px 24px rgba(0,0,0,0.06);
}
.compare-table th, .compare-table td {
  padding: 16px 20px; text-align: left; font-size: 14px;
  border-bottom: 1px solid #f3f4f6;
}
.compare-table thead th { background: #f9fafb; font-weight: 700; font-size: 13px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px; }
.compare-table th.us, .compare-table td.us {
  background: linear-gradient(135deg, rgba(102,126,234,0.06), rgba(118,75,162,0.06));
  color: #4338ca; font-weight: 600;
}
.row-label { font-weight: 600; color: #1f2937; }
.them { color: #9ca3af; }

/* FAQ */
.faq { padding: 100px 24px; max-width: 760px; margin: 0 auto; }
.faq-list { display: flex; flex-direction: column; gap: 14px; }
.faq-item {
  background: #fff; border: 1.5px solid #e5e7eb;
  border-radius: 14px; padding: 22px 26px;
  transition: border-color 0.2s;
}
.faq-item:hover { border-color: #a5b4fc; }
.faq-item summary {
  cursor: pointer; font-weight: 700; color: #1f2937;
  list-style: none; font-size: 15px; line-height: 1.5;
}
.faq-item summary::-webkit-details-marker { display: none; }
.faq-item[open] { border-color: #a5b4fc; }
.faq-item[open] summary { margin-bottom: 14px; color: #6366f1; }
.faq-item p { margin: 0; color: #6b7280; line-height: 1.8; font-size: 14px; }
.faq-item code {
  background: #f3f4f6; padding: 2px 7px; border-radius: 5px;
  font-family: 'SF Mono', Menlo, monospace; font-size: 13px; color: #4338ca;
}

/* CTA bottom */
.cta-bottom {
  padding: 100px 24px; text-align: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff; position: relative; overflow: hidden;
}
.cta-bottom::before {
  content: '';
  position: absolute; inset: 0;
  background: radial-gradient(circle at 30% 50%, rgba(255,255,255,0.1) 0%, transparent 60%);
}
.cta-bottom h2 { font-size: 36px; margin: 0 0 12px; color: #fff; font-weight: 800; position: relative; }
.cta-bottom p { margin: 0 0 32px; color: rgba(255,255,255,0.85); font-size: 16px; position: relative; }
.cta-bottom .btn-cta-primary {
  background: #fff; color: #6366f1;
  box-shadow: 0 8px 32px rgba(0,0,0,0.15);
  position: relative;
}

/* Footer */
.footer { background: #0f172a; color: #94a3b8; padding: 72px 24px 36px; }
.footer-inner {
  max-width: 1100px; margin: 0 auto;
  display: grid; grid-template-columns: 2fr 1fr 1fr 1fr; gap: 48px;
}
.footer-brand .logo { color: #a5b4fc; font-size: 20px; margin-bottom: 12px; }
.footer-brand p { margin: 0; font-size: 13px; }
.footer-meta { color: #475569; margin-top: 8px !important; }
.footer-col h4 { color: #f1f5f9; font-size: 14px; font-weight: 600; margin: 0 0 16px; }
.footer-col a {
  display: block; color: #94a3b8; font-size: 13px;
  text-decoration: none; padding: 5px 0; transition: color 0.2s;
}
.footer-col a:hover { color: #f1f5f9; }
.footer-bottom {
  max-width: 1100px; margin: 48px auto 0;
  padding-top: 24px; border-top: 1px solid #1e293b;
  text-align: center; font-size: 12px; color: #475569;
}

/* Mobile */
@media (max-width: 720px) {
  .hero-title { font-size: 34px; }
  .hero { padding: 70px 20px 90px; }
  .nav-links { display: none; }
  .hero-trust { gap: 14px; font-size: 12px; }
  .footer-inner { grid-template-columns: 1fr; gap: 32px; }
  .price-card.featured { transform: none; }
  .compare-table th, .compare-table td { padding: 10px 10px; font-size: 12px; }
  .features { padding: 70px 20px; }
  .pricing { padding: 70px 20px; }
  .faq { padding: 70px 20px; }
  .section-title { font-size: 26px; }
}
</style>
