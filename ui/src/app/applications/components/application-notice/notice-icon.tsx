import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {parseNotice, shouldShowIcon, tooltipPreview} from './notice';
import './notice.scss';

interface NoticeIconProps {
    annotations: {[key: string]: string} | undefined;
}

export const NoticeIcon = ({annotations}: NoticeIconProps) => {
    const notice = parseNotice(annotations);
    if (!notice || !shouldShowIcon(notice)) {
        return null;
    }
    const preview = tooltipPreview(notice.content);
    return (
        <Tooltip content={preview}>
            <i
                className={`fa ${notice.iconClass} application-notice-icon application-notice-icon--${notice.severity}`}
                aria-label={`Notice: ${preview}`}
                role='img'
                onClick={e => e.stopPropagation()}
            />
        </Tooltip>
    );
};
