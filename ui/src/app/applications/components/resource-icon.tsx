import * as React from 'react';
import {resources} from './resources';

export const ResourceIcon = ({kind}: {kind: string}) => {
    const i = resources.get(kind);
    if (i !== undefined) {
        return <img src={'assets/images/resources/' + i + '.svg'} alt={kind} style={{padding: '2px', width: '40px', height: '32px'}} />;
    }
    if (kind === 'Application') {
        return <i title={kind} className={`icon argo-icon-application`} />;
    }
    const initials = kind.replace(/[a-z]/g, '');
    const n = initials.length;
    return (
        <div style={{display: 'inline-block', padding: '2px 4px', width: '40px', height: '32px'}}>
            <i title={kind} className='icon fa fa-circle' />
            <div
                style={{
                    position: 'absolute',
                    left: '10px',
                    top: `${n <= 2 ? 10 : 14}px`,
                    width: '40px',
                    margin: 'auto',
                    color: 'white',
                    textAlign: 'center',
                    fontSize: `${n <= 2 ? 1 : 0.6}em`
                }}>
                {initials}
            </div>
        </div>
    );
};
