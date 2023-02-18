import i18next from 'i18next';
import en from './locales/en';
import ko from './locales/ko';

i18next.init({
    lng: navigator.language,
    fallbackLng: 'en',
    debug: process.env.NODE_ENV !== 'production',
    resources: {
        en: {translation: en},
        ko: {translation: ko}
    }
});
