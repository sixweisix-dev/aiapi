<template>
  <div class="social-wrap">
    <div class="social-divider">
      <span>{{ t('login.orDivider') }}</span>
    </div>
    <a href="/v1/auth/github/start" class="social-btn github">
      <svg viewBox="0 0 24 24" width="18" height="18">
        <path fill="currentColor" d="M12 .5C5.7.5.5 5.7.5 12c0 5.1 3.3 9.4 7.9 10.9.6.1.8-.3.8-.6v-2c-3.2.7-3.9-1.5-3.9-1.5-.5-1.3-1.3-1.7-1.3-1.7-1.1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1 1.8 2.7 1.3 3.4 1 .1-.8.4-1.3.7-1.6-2.6-.3-5.3-1.3-5.3-5.7 0-1.3.4-2.3 1.2-3.1-.1-.3-.5-1.5.1-3.1 0 0 1-.3 3.2 1.2.9-.3 1.9-.4 2.9-.4 1 0 2 .1 2.9.4 2.2-1.5 3.2-1.2 3.2-1.2.6 1.6.2 2.8.1 3.1.7.8 1.2 1.8 1.2 3.1 0 4.4-2.7 5.4-5.3 5.7.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6 4.6-1.5 7.9-5.8 7.9-10.9C23.5 5.7 18.3.5 12 .5z"/>
      </svg>
      <span>{{ t('login.githubBtn') }}</span>
    </a>
    <a href="/v1/auth/google/start" class="social-btn google">
      <svg viewBox="0 0 48 48" width="18" height="18">
        <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
        <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
        <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
        <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
      </svg>
      <span>{{ t('login.googleBtn') }}</span>
    </a>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>


<style scoped>
.social-wrap { margin: 16px 0 8px; }
.social-divider {
  display: flex; align-items: center; margin: 12px 0;
  color: #9ca3af; font-size: 12px;
}
.social-divider::before, .social-divider::after {
  content: ''; flex: 1; height: 1px; background: #e5e7eb;
}
.social-divider span { padding: 0 12px; }
.social-btn {
  display: flex; align-items: center; justify-content: center; gap: 10px;
  width: 100%; height: 44px; border-radius: 12px; text-decoration: none;
  font-size: 14px; font-weight: 600;
  transition: transform .1s;
}
.social-btn:active { transform: scale(0.98); }
.social-btn.github {
  background: #24292f; color: #fff;
}
.social-btn.google {
  background: #fff; color: #3c4043;
  border: 1px solid #dadce0;
}
.social-btn + .social-btn { margin-top: 8px; }
</style>
