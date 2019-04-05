import { DropDownMenu } from 'argo-ui';
import * as React from 'react';

import { LoadBalancerIngress } from '../../shared/models';

export const ApplicationIngressLink = ({ingress}: { ingress: LoadBalancerIngress[]}) => {
    const items = (ingress || []).map((item) => item.hostname || item.ip).filter((item) => !!item);
    return items.length > 0 && (
        <span>
            <a onClick={(e) => {
                e.stopPropagation();
                window.open(`https://${items[0]}`);
            }}>
                <i className='fa fa-external-link-alt'/> {items.length > 1 && <DropDownMenu anchor={() => <i className='fa fa-caret-down'/>} items={items.map((item) => ({
                    title: item,
                    action: () => window.open(`https://${item}`),
                }))} />}
            </a>
        </span>
    ) || null;
};
