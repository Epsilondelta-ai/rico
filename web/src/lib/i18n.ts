import { addMessages, init, getLocaleFromNavigator, locale } from 'svelte-i18n';

import ko from '../locales/ko.json';
import en from '../locales/en.json';

// 메시지 등록
addMessages('ko', ko);
addMessages('en', en);

// localStorage에서 저장된 언어 불러오기
const LOCALE_KEY = 'rico_locale';
function getSavedLocale(): string | null {
  try {
    return localStorage.getItem(LOCALE_KEY);
  } catch {
    return null;
  }
}

// 언어 저장
export function saveLocale(lang: string) {
  try {
    localStorage.setItem(LOCALE_KEY, lang);
  } catch {
    // 무시
  }
}

// 초기화
init({
  fallbackLocale: 'en',
  initialLocale: getSavedLocale() || getLocaleFromNavigator() || 'en',
});

// 언어 변경 함수
export function setLocale(lang: string) {
  locale.set(lang);
  saveLocale(lang);
}

// 현재 언어 가져오기
export { locale };

// 현재 locale 값 가져오기 (동기)
export function getCurrentLocale(): string {
  return getSavedLocale() || getLocaleFromNavigator() || 'en';
}
