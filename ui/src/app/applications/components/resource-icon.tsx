import * as React from 'react';
import {resources} from './resources';

export const ResourceIcon = ({kind, icon}: {kind: string; icon?: string}) => {
    const i = resources.get(kind);
    if (i !== undefined) {
        return <img src={'assets/images/resources/' + i + '.svg'} alt={kind} style={{padding: '2px', width: '40px', height: '32px'}} />;
    }
    if (kind === 'Application') {
        if (icon !== null || icon !== undefined || icon !== '') {
            return (
                <img
                    src={icon}
                    alt={kind}
                    style={{
                        position: 'absolute',
                        top: '50%',
                        left: '50%',
                        width: '40px',
                        height: '40px',
                        marginTop: '-20px',
                        marginLeft: '-20px'
                    }}
                />
            );
        } else {
            return <i title={kind} className={`icon argo-icon-application`} />;
        }
    }
    const initials = kind.replace(/[a-z]/g, '');
    const n = initials.length;
    const style: React.CSSProperties = {
        display: 'inline-block',
        verticalAlign: 'middle',
        padding: '2px 4px',
        width: '32px',
        height: '32px',
        borderRadius: '50%',
        backgroundColor: '#8FA4B1',
        textAlign: 'center',
        lineHeight: '30px'
    };
    return (
        <div style={style}>
            <span style={{color: 'white', fontSize: `${n <= 2 ? 1 : 0.6}em`}}>{initials}</span>
        </div>
    );
};
