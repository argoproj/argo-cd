import * as React from 'react';
import {useTranslation} from 'react-i18next';
import {languages} from '../../i18n';
import en from '../../locales/en';

interface Props {
    title: string;
}

export const LanguageSelector = ({title}: Props) => {
    const {t, i18n} = useTranslation();

    return (
        <div style={{display: 'flex', flexDirection: 'row', gap: '20px'}}>
            {title}

            <label>
                {t('languages', en['languages'])}
                <select
                    value={localStorage.getItem('language') || navigator.language}
                    className='argo-select'
                    style={{width: 'auto', padding: '8px'}}
                    onChange={e => {
                        i18n.changeLanguage(e.currentTarget.value);
                        localStorage.setItem('language', e.currentTarget.value);
                    }}>
                    {languages.map(({name, value}) => (
                        <option key={value} value={value}>
                            {name}
                        </option>
                    ))}
                </select>
            </label>
        </div>
    );
};
