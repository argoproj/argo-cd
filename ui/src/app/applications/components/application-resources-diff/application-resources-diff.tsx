import {Checkbox, DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import 'react-diff-view/style/index.css';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationResourcesDiffItem, clearQueue, disableQueue, enableQueue} from './application-resources-diff-item';

import './application-resources-diff.scss';

export interface ApplicationResourcesDiffProps {
    states: models.ResourceDiff[];
}

export const ApplicationResourcesDiff = (props: ApplicationResourcesDiffProps) => {
    React.useEffect(() => {
        enableQueue();
        return () => {
            clearQueue();
            disableQueue();
        };
    }, []);
    return (
        <DataLoader key='resource-diff' load={() => services.viewPreferences.getPreferences()}>
            {pref => {
                const diffTextPrepare = props.states
                    .map(state => {
                        return {
                            a: state.normalizedLiveState ? jsYaml.dump(state.normalizedLiveState, {indent: 2}) : '',
                            b: state.predictedLiveState ? jsYaml.dump(state.predictedLiveState, {indent: 2}) : '',
                            hook: state.hook,
                            // doubles as sort order
                            name: (state.group || '') + '/' + state.kind + '/' + (state.namespace ? state.namespace + '/' : '') + state.name
                        };
                    })
                    .filter(i => !i.hook)
                    .filter(i => i.a !== i.b);

                // assume that if you only have one file, we don't need the file path
                const whiteBox = props.states.length > 1 ? 'white-box' : '';
                const showPath = props.states.length > 1;

                return (
                    <div className='application-resources-diff'>
                        <div className={whiteBox + ' application-resources-diff__checkboxes'}>
                            <Checkbox
                                id='compactDiff'
                                checked={pref.appDetails.compactDiff}
                                onChange={() => {
                                    clearQueue();
                                    enableQueue();
                                    services.viewPreferences.updatePreferences({
                                        appDetails: {
                                            ...pref.appDetails,
                                            compactDiff: !pref.appDetails.compactDiff
                                        }
                                    });
                                }}
                            />
                            <label htmlFor='compactDiff'>Compact diff</label>
                            <Checkbox
                                id='inlineDiff'
                                checked={pref.appDetails.inlineDiff}
                                onChange={() => {
                                    clearQueue();
                                    enableQueue();
                                    services.viewPreferences.updatePreferences({
                                        appDetails: {
                                            ...pref.appDetails,
                                            inlineDiff: !pref.appDetails.inlineDiff
                                        }
                                    });
                                }}
                            />
                            <label htmlFor='inlineDiff'>Inline diff</label>
                        </div>
                        {diffTextPrepare
                            .sort((a, b) => a.name.localeCompare(b.name))
                            .map(item => (
                                <ApplicationResourcesDiffItem
                                    key={item.name}
                                    a={item.a}
                                    b={item.b}
                                    name={item.name}
                                    hook={item.hook}
                                    showPath={showPath}
                                    whiteBox={whiteBox}
                                    compactDiff={pref.appDetails.compactDiff}
                                    inlineDiff={pref.appDetails.inlineDiff}></ApplicationResourcesDiffItem>
                            ))}
                    </div>
                );
            }}
        </DataLoader>
    );
};
