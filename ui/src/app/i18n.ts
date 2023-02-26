import i18next from 'i18next';
import {initReactI18next} from 'react-i18next';

import en from './locales/en';
import ko from './locales/ko';

i18next.use(initReactI18next).init({
    lng: navigator.language,
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
