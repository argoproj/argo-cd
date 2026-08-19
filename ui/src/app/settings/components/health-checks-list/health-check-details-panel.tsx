import * as React from 'react';
import {SlidingPanel} from 'argo-ui';
import * as models from '../../../shared/models';
import {MonacoEditor} from '../../../shared/components/monaco-editor';
import {renderOriginBadge} from './health-checks-list';

export interface HealthCheckDetailsPanelProps {
    item: models.HealthCheckItem | null;
    onClose: () => void;
}

export const HealthCheckDetailsPanel: React.FC<HealthCheckDetailsPanelProps> = ({item, onClose}) => {
    const isShown = item !== null;

    if (!item) {
        return <SlidingPanel isShown={false} onClose={onClose} />;
    }

    const isLuaOrigin = item.origin === 'BuiltinLua' || item.origin === 'CustomLua' || item.origin === 'OverrideLua';
    const isGoOrigin = item.origin === 'BuiltinGo';

    return (
        <SlidingPanel
            isShown={isShown}
            onClose={onClose}
            header={
                <div className='health-check-details-panel__header'>
                    <h4>{item.group ? `${item.group}/${item.kind}` : item.kind}</h4>
                    <button className='argo-button argo-button--base-o' onClick={onClose}>
                        Close
                    </button>
                </div>
            }>
            <div className='health-check-details-panel__content' style={{padding: '1.5em'}}>
                <div className='white-box' style={{marginBottom: '1.5em'}}>
                    <div className='white-box__details'>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3 medium-2 bold'>Group:</div>
                            <div className='columns small-9 medium-10'>{item.group || '(core / empty)'}</div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3 medium-2 bold'>Kind:</div>
                            <div className='columns small-9 medium-10'>{item.kind}</div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3 medium-2 bold'>Key:</div>
                            <div className='columns small-9 medium-10'>
                                <code>{item.key}</code>
                            </div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3 medium-2 bold'>Origin:</div>
                            <div className='columns small-9 medium-10'>{renderOriginBadge(item.origin)}</div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3 medium-2 bold'>Wildcard:</div>
                            <div className='columns small-9 medium-10'>
                                {item.isWildcard ? (
                                    <span className='health-checks-list__wildcard-tag'>
                                        <i className='fa fa-asterisk' /> Yes
                                    </span>
                                ) : (
                                    'No'
                                )}
                            </div>
                        </div>
                        <div className='row white-box__details-row'>
                            <div className='columns small-3 medium-2 bold'>Open Libraries:</div>
                            <div className='columns small-9 medium-10'>{item.useOpenLibs ? 'Enabled' : 'Disabled'}</div>
                        </div>
                    </div>
                </div>

                <div className='health-check-details-panel__source'>
                    <h5>Health Check Source</h5>
                    {isGoOrigin && (
                        <div className='argo-notification-panel argo-notification-panel--info' style={{marginTop: '1em'}}>
                            <i className='fa fa-info-circle' style={{marginRight: '0.5em'}} />
                            This health check is implemented natively in Go within <strong>gitops-engine</strong> and does not have a Lua script source.
                        </div>
                    )}
                    {isLuaOrigin && (
                        <div style={{marginTop: '1em', border: '1px solid #e8e8e8', borderRadius: '4px', overflow: 'hidden'}}>
                            {item.luaScript ? (
                                <MonacoEditor
                                    minHeight={300}
                                    vScrollBar={true}
                                    editor={{
                                        input: {text: item.luaScript, language: 'lua'},
                                        options: {readOnly: true}
                                    }}
                                />
                            ) : (
                                <div className='p-3 text-muted' style={{padding: '1em'}}>
                                    No Lua script source code available for this health check.
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </SlidingPanel>
    );
};
