import { DropDownMenu } from 'argo-ui';
import * as React from 'react';

export const ApplicationURLs = ({urls}: { urls: string[]}) => {
    return (urls || []).length > 0 && (
        <span>
            <a onClick={(e) => {
                e.stopPropagation();
                window.open(urls[0]);
            }}>
                <i className='fa fa-external-link-alt'/> {urls.length > 1 && <DropDownMenu anchor={() => <i className='fa fa-caret-down'/>} items={urls.map((item) => ({
                    title: item,
                    action: () => window.open(item),
                }))} />}
            </a>
        </span>
    ) || null;
};
