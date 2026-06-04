import {Toolbar, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Observable} from 'rxjs';
import {AddAuthToToolbar, DataLoader} from '../';
import {Context} from '../../context';

import './flex-top-bar.scss';

interface FlexTopBarProps {
    toolbar: Toolbar | Observable<Toolbar>;
}

export const FlexTopBar = (props: FlexTopBarProps) => {
    const ctx = React.useContext(Context);
    const loadToolbar = AddAuthToToolbar(props.toolbar, ctx);

    return (
        <React.Fragment>
            <div className='top-bar row flex-top-bar' key='tool-bar'>
                <DataLoader load={() => loadToolbar}>
                    {toolbar => <FlexTopBarContent toolbar={toolbar} />}
                </DataLoader>
            </div>
            <div className='flex-top-bar__padder' />
        </React.Fragment>
    );
};

const FlexTopBarContent = (props: {toolbar: Toolbar}) => {
    return (
        <React.Fragment>
            <div className='flex-top-bar__actions'>
                {props.toolbar.actionMenu && (
                    <React.Fragment>
                        {props.toolbar.actionMenu.items.map((item, i) => (
                            <Tooltip className='custom-tooltip' content={item.title} key={item.qeId || i}>
                                <button
                                    disabled={!!item.disabled}
                                    qe-id={item.qeId}
                                    className='argo-button argo-button--base'
                                    onClick={() => item.action()}
                                    style={{marginRight: 2}}
                                    key={i}>
                                    {item.iconClassName && <i className={item.iconClassName} style={{marginLeft: '-5px', marginRight: '5px'}} />}
                                    <span className='show-for-large'>{item.title}</span>
                                </button>
                            </Tooltip>
                        ))}
                    </React.Fragment>
                )}
            </div>
            <div className='flex-top-bar__tools'>
                {Array.isArray(props.toolbar.tools) ? (
                    <>
                        <div style={{flexGrow: 1}}>{props.toolbar.tools[0]}</div>
                        {props.toolbar.tools[1]}
                    </>
                ) : (
                    props.toolbar.tools
                )}
            </div>
        </React.Fragment>
    );
};
