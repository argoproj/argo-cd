import * as React from 'react';
import * as models from '../../../shared/models';
import './application-icon.scss';

interface ApplicationIconProps {
    app: models.Application;
    size?: 'small' | 'medium' | 'large';
}

export const isValidIconUrl = (url: string): boolean => {
    if (!url.startsWith('https://')) {
        return false;
    }
    try {
        new URL(url);
        return true;
    } catch {
        return false;
    }
};

export const ApplicationIcon = ({app, size = 'medium'}: ApplicationIconProps) => {
    const [imgError, setImgError] = React.useState(false);
    const annotations = app.metadata.annotations || {};
    const iconUrl = annotations['argocd.argoproj.io/icon'];
    const source = app.spec.sources?.length > 0 ? app.spec.sources[0] : app.spec.source;
    const isOci = source?.repoURL?.startsWith('oci://');
    const fallbackClass = 'icon argo-icon-' + (source?.chart != null ? 'helm' : isOci ? 'oci' : 'git');

    if (iconUrl && isValidIconUrl(iconUrl) && !imgError) {
        return <img src={iconUrl} alt='Application icon' className={`application-icon application-icon--${size}`} onError={() => setImgError(true)} />;
    }
    return <i className={fallbackClass} />;
};
