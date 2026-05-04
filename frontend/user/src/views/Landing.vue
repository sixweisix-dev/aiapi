<template>
  <div class="landing">
    <!-- 顶部导航 -->
    <header class="navbar">
      <div class="nav-inner">
        <div class="logo">⚡ TransitAI</div>
        <nav class="nav-links">
          <a href="#pricing">{{ t('landing.navPricing') }}</a>
          <a href="#compare">{{ t('landing.navCompare') }}</a>
          <a href="#faq">FAQ</a>
        </nav>
        <div class="nav-actions">
          <button class="lang-switch" @click="toggleLang">{{ lang === 'zh' ? 'EN' : '中' }}</button>
          <router-link to="/login" class="btn-ghost">{{ t('landing.navLogin') }}</router-link>
          <router-link to="/register" class="btn-primary">{{ t('landing.navRegister') }}</router-link>
        </div>
      </div>
    </header>

    <!-- Hero -->
    <section class="hero">
      <div class="hero-inner">
        <div class="badge">{{ t('landing.heroBadge') }}</div>
        <h1 class="hero-title">
          {{ t('landing.heroTitle1') }} <span class="hl">{{ t('landing.heroTitleHl') }}</span> {{ t('landing.heroTitle2') }}
        </h1>
        <p class="hero-subtitle">
          {{ t('landing.heroSub1') }}<strong> {{ t('landing.heroSubBold') }}</strong>。<br/>
          {{ t('landing.heroSub2') }}
        </p>
        <div class="hero-cta">
          <router-link to="/register" class="btn-cta-primary">{{ t('landing.heroCtaTry') }}</router-link>
          <a href="#pricing" class="btn-cta-ghost">{{ t('landing.heroCtaPricing') }}</a>
        </div>
        <div class="hero-trust">
          <span>{{ t('landing.trust1') }}</span>
          <span>{{ t('landing.trust2') }}</span>
          <span>{{ t('landing.trust3') }}</span>
          <span>{{ t('landing.trust4') }}</span>
        </div>
      </div>
    </section>

    <!-- 核心优势 -->
    <section class="features">
      <h2 class="section-title">{{ t('landing.featuresTitle') }}</h2>
      <p class="section-sub">{{ t('landing.featuresSub') }}</p>
      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">🎯</div>
          <h3>{{ t('landing.feat1Title') }}</h3>
          <p>{{ t('landing.feat1Desc') }}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">⚡</div>
          <h3>{{ t('landing.feat2Title') }}</h3>
          <p>{{ t('landing.feat2Desc') }}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">📊</div>
          <h3>{{ t('landing.feat3Title') }}</h3>
          <p>{{ t('landing.feat3Desc') }}</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">🔒</div>
          <h3>{{ t('landing.feat4Title') }}</h3>
          <p>{{ t('landing.feat4Desc') }}</p>
        </div>
      </div>
    </section>

    <!-- 价格表 -->
    <section id="pricing" class="pricing" style="scroll-margin-top: 80px">
      <h2 class="section-title">{{ t('landing.pricingTitle') }}</h2>
      <p class="section-sub">{{ t('landing.pricingSub') }}</p>

      <div class="pricing-grid">
        <!-- 入门版 -->
        <div class="price-card">
          <div class="price-header">
            <h3>{{ t('landing.starterName') }}</h3>
            <p class="price-tagline">{{ t('landing.starterTag') }}</p>
          </div>
          <div class="price-amount">
            <span class="price-num">{{ t('landing.starterPrice') }}</span>
            <span class="price-unit">{{ t('landing.starterUnit') }}</span>
          </div>
          <ul class="price-features">
            <li>{{ t('landing.starterF1') }}</li>
            <li>{{ t('landing.starterF2') }}</li>
            <li>{{ t('landing.starterF3') }}</li>
            <li>{{ t('landing.starterF4') }}</li>
            <li>{{ t('landing.starterF5') }}</li>
            <li>{{ t('landing.starterF6') }}</li>
          </ul>
          <router-link to="/register" class="price-cta">{{ t('landing.starterCta') }}</router-link>
        </div>

        <!-- 专业版（推荐）-->
        <div class="price-card featured">
          <div class="featured-badge">{{ t('landing.proBadge') }}</div>
          <div class="price-header">
            <h3>{{ t('landing.proName') }}</h3>
            <p class="price-tagline">{{ t('landing.proTag') }}</p>
          </div>
          <div class="price-amount">
            <span class="price-num">¥99</span>
            <span class="price-unit">{{ t('landing.proUnit') }}</span>
          </div>
          <ul class="price-features">
            <li>{{ t('landing.proF1Pre') }} <strong>{{ t('landing.proF1Bold') }}</strong></li>
            <li>{{ t('landing.proF2') }}</li>
            <li>{{ t('landing.proF3') }}</li>
            <li>{{ t('landing.proF4') }}</li>
            <li>{{ t('landing.proF5') }}</li>
            <li>{{ t('landing.proF6') }}</li>
          </ul>
          <router-link to="/register" class="price-cta primary">{{ t('landing.proCta') }}</router-link>
        </div>

        <!-- 企业版 -->
        <div class="price-card">
          <div class="price-header">
            <h3>{{ t('landing.entName') }}</h3>
            <p class="price-tagline">{{ t('landing.entTag') }}</p>
          </div>
          <div class="price-amount">
            <span class="price-num">¥499</span>
            <span class="price-unit">{{ t('landing.entUnit') }}</span>
          </div>
          <ul class="price-features">
            <li>{{ t('landing.entF1Pre') }} <strong>{{ t('landing.entF1Bold') }}</strong></li>
            <li>{{ t('landing.entF2') }}</li>
            <li>{{ t('landing.entF3') }}</li>
            <li>{{ t('landing.entF4') }}</li>
            <li>{{ t('landing.entF5') }}</li>
            <li>{{ t('landing.entF6') }}</li>
          </ul>
          <a href="mailto:sixweisix@gmail.com" class="price-cta">{{ t('landing.entCta') }}</a>
        </div>
      </div>

      <p class="pricing-note">
        {{ t('landing.pricingNote') }}
      </p>
    </section>

    <!-- 对比表 -->
    <section id="compare" class="compare" style="scroll-margin-top: 80px">
      <h2 class="section-title">{{ t('landing.compareTitle') }}</h2>
      <p class="section-sub">{{ t('landing.compareSub') }}</p>

      <div class="compare-table-wrap">
        <table class="compare-table">
          <thead>
            <tr>
              <th></th>
              <th class="us">{{ t('landing.compareUs') }}</th>
              <th>{{ t('landing.compareThem') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow1') }}</td>
              <td class="us">{{ t('landing.cmpUs1') }}</td>
              <td class="them">{{ t('landing.cmpThem1') }}</td>
            </tr>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow2') }}</td>
              <td class="us">{{ t('landing.cmpUs2') }}</td>
              <td class="them">{{ t('landing.cmpThem2') }}</td>
            </tr>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow3') }}</td>
              <td class="us">{{ t('landing.cmpUs3') }}</td>
              <td class="them">{{ t('landing.cmpThem3') }}</td>
            </tr>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow4') }}</td>
              <td class="us">{{ t('landing.cmpUs4') }}</td>
              <td class="them">{{ t('landing.cmpThem4') }}</td>
            </tr>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow5') }}</td>
              <td class="us">{{ t('landing.cmpUs5') }}</td>
              <td class="them">{{ t('landing.cmpThem5') }}</td>
            </tr>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow6') }}</td>
              <td class="us">{{ t('landing.cmpUs6') }}</td>
              <td class="them">{{ t('landing.cmpThem6') }}</td>
            </tr>
            <tr>
              <td class="row-label">{{ t('landing.cmpRow7') }}</td>
              <td class="us">{{ t('landing.cmpUs7') }}</td>
              <td class="them">{{ t('landing.cmpThem7') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- FAQ -->
    <section id="faq" class="faq" style="scroll-margin-top: 80px">
      <h2 class="section-title">{{ t('landing.faqTitle') }}</h2>
      <div class="faq-list">
        <details class="faq-item">
          <summary>{{ t('landing.faqQ1') }}</summary>
          <p>{{ t('landing.faqA1') }}</p>
        </details>
        <details class="faq-item">
          <summary>{{ t('landing.faqQ2') }}</summary>
          <p>{{ t('landing.faqA2Pre') }}<strong>{{ t('landing.faqA2Bold') }}</strong>{{ t('landing.faqA2Post') }}</p>
        </details>
        <details class="faq-item">
          <summary>{{ t('landing.faqQ3') }}</summary>
          <p>{{ t('landing.faqA3') }}</p>
        </details>
        <details class="faq-item">
          <summary>{{ t('landing.faqQ4') }}</summary>
          <p>{{ t('landing.faqA4') }}</p>
        </details>
        <details class="faq-item">
          <summary>{{ t('landing.faqQ5') }}</summary>
          <p>{{ t('landing.faqA5Pre') }}<code>chat/completions</code>{{ t('landing.faqA5Post') }}</p>
        </details>
        <details class="faq-item">
          <summary>{{ t('landing.faqQ6') }}</summary>
          <p>{{ t('landing.faqA6') }}</p>
        </details>
      </div>
    </section>

    <!-- 底部 CTA -->
    <section class="cta-bottom">
      <h2>{{ t('landing.ctaBottomTitle') }}</h2>
      <p>{{ t('landing.ctaBottomSub') }}</p>
      <router-link to="/register" class="btn-cta-primary">{{ t('landing.ctaBottomBtn') }}</router-link>
    </section>

    <!-- Footer -->
    <footer class="footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <div class="logo">⚡ TransitAI</div>
          <p class="footer-meta">{{ t('landing.footerTagline') }}</p>
        </div>
        <div class="footer-col">
          <h4>{{ t('landing.footerProduct') }}</h4>
          <a href="#pricing">{{ t('landing.navPricing') }}</a>
          <a href="#compare">{{ t('landing.navCompare') }}</a>
          <router-link to="/login">{{ t('landing.navLogin') }}</router-link>
          <router-link to="/register">{{ t('landing.navRegister') }}</router-link>
        </div>
        <div class="footer-col">
          <h4>{{ t('landing.footerTerms') }}</h4>
          <router-link to="/terms">{{ t('landing.footerTermsLink') }}</router-link>
          <router-link to="/privacy">{{ t('landing.footerPrivacyLink') }}</router-link>
        </div>
        <div class="footer-col">
          <h4>{{ t('landing.footerContact') }}</h4>
          <a href="mailto:sixweisix@gmail.com">sixweisix@gmail.com</a>
        </div>
      </div>
      <div class="footer-bottom">
        {{ t('landing.footerCopyright') }}
      </div>
    </footer>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { setLocale, currentLocale } from '@/i18n'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const lang = ref(currentLocale())
function toggleLang() {
  const next = lang.value === 'zh' ? 'en' : 'zh'
  setLocale(next)
  lang.value = next
}
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
  display: flex; align-items: center; gap: 36px;
  padding: 14px 24px;
}
.nav-actions { margin-left: auto; }
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

.lang-switch {
  background: rgba(102,126,234,0.1);
  color: #667eea;
  border: none;
  border-radius: 8px;
  padding: 6px 12px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  margin-right: 4px;
}
.lang-switch:hover { background: rgba(102,126,234,0.2); }
</style>
