import * as React from 'react';
import {FormApi} from 'react-form';
import * as models from '../../../shared/models';
import {CreatePanelSourceTypeParameters} from './create-panel-source-type-parameters';
import {SourcePanel} from './source-panel';

export function CollapsibleMultiSourceSection(props: {
    index: number;
    formApi: FormApi;
    repos: string[];
    reposInfo: models.Repository[];
    formApp: models.Application;
    canRemove?: boolean;
    onRemove?: () => void;
}) {
    const [expanded, setExpanded] = React.useState(true);
    const src = props.formApp.spec.sources?.[props.index];
    const repoInfoFor = props.reposInfo.find(r => r.repo === src?.repoURL);
    const title = `Source ${props.index + 1}${src?.name ? ` — ${src.name}` : ''}: ${src?.repoURL || ''}`;
    const desc = [src?.path && `PATH=${src.path}`, src?.chart && `CHART=${src.chart}`, src?.targetRevision && `REVISION=${src.targetRevision}`].filter(Boolean).join(', ');

    if (!expanded) {
        return (
            <div className='settings-overview__redirect-panel application-create-panel__multi-source-collapsed' onClick={() => setExpanded(true)}>
                <div className='editable-panel__collapsible-button'>
                    <i className='fa fa-angle-down filter__collapse editable-panel__collapsible-button__override' />
                </div>
                <div className='settings-overview__redirect-panel__content'>
                    <div className='settings-overview__redirect-panel__title'>{title}</div>
                    <div className='settings-overview__redirect-panel__description'>{desc}</div>
                </div>
            </div>
        );
    }

    return (
        <div className='white-box application-create-panel__multi-source-section'>
            <div className='application-create-panel__multi-source-header'>
                <span className='application-create-panel__multi-source-label'>Source {props.index + 1}</span>
                <div className='application-create-panel__multi-source-header-actions'>
                    {props.canRemove && props.onRemove && (
                        <button
                            type='button'
                            className='argo-button argo-button--base application-create-panel__multi-source-remove-btn'
                            title='Remove this source'
                            onClick={e => {
                                e.stopPropagation();
                                props.onRemove?.();
                            }}>
                            <i className='fa fa-minus' style={{marginLeft: '-5px', marginRight: '5px'}} />
                            Remove source
                        </button>
                    )}
                    <button
                        type='button'
                        className='application-create-panel__multi-source-collapse-btn'
                        title='Collapse source'
                        aria-label='Collapse source section'
                        onClick={() => setExpanded(false)}>
                        <i className='fa fa-angle-up' />
                    </button>
                </div>
            </div>
            <div className='application-create-panel__multi-source-block'>
                <SourcePanel formApi={props.formApi} repos={props.repos} repoInfo={repoInfoFor} sourceIndex={props.index} suppressMultiSourceHeading={true} />
            </div>
            <div className='application-create-panel__multi-source-params'>
                <CreatePanelSourceTypeParameters formApi={props.formApi} sourceIndex={props.index} />
            </div>
        </div>
    );
}
