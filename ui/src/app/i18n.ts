import i18next from 'i18next';
import {initReactI18next} from 'react-i18next';

import en from './locales/en';
import ko from './locales/ko';

export const languages = [
    {
        name: 'English',
        value: 'en'
    },
    {
        name: '한국어',
        value: 'ko'
    }
];

i18next.use(initReactI18next).init({
    lng: localStorage.getItem('language') || navigator.language,
    fallbackLng: 'en',
    debug: process.env.NODE_ENV !== 'production',
    resources: {
        en: {translation: en},
        ko: {translation: ko}
    },
    interpolation: {
        escapeValue: false
    }
});
