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
    return (
        <div style={{display: 'inline-block', padding: '2px 4px', width: '40px', height: '32px'}}>
            <i title={kind} className='icon fa fa-circle' />
            <div
                style={{
                    position: 'absolute',
                    left: '10px',
                    top: '10px',
                    width: '40px',
                    margin: 'auto',
                    color: 'white',
                    textAlign: 'center'
                }}>
                {kind.replace(/[a-z]/g, '').substring(0, 2)}
            </div>
        </div>
    );
};
