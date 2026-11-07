import classNames from 'classnames';
import * as React from 'react';

import * as models from '../../../shared/models';
import {countHiddenSelected, deselectHidden, KindCount} from './application-sync-panel-utils';

interface KindFilterProps {
    resources: models.ResourceStatus[];
    selection: boolean[];
    kindCounts: KindCount[];
    kinds: string[];
    onKindsChange: (kinds: string[]) => void;
    onSelectionChange: (selection: boolean[]) => void;
}

export const ApplicationSyncPanelKindFilter = ({resources, selection, kindCounts, kinds, onKindsChange, onSelectionChange}: KindFilterProps) => {
    // Nothing to filter by when the application has a single kind.
    if (kindCounts.length < 2) {
        return null;
    }
    const hiddenSelected = countHiddenSelected(resources, selection, kinds);
    return (
        <div className='application-sync-panel__kinds'>
            <div role='group' aria-label='Filter resources by kind'>
                {kindCounts.map(({kind, count}) => {
                    const active = kinds.includes(kind);
                    return (
                        <button
                            key={kind}
                            type='button'
                            aria-pressed={active}
                            className={classNames('application-sync-panel__kind', {'application-sync-panel__kind--active': active})}
                            onClick={() => onKindsChange(active ? kinds.filter(k => k !== kind) : [...kinds, kind])}>
                            {kind} ({count})
                        </button>
                    );
                })}
                {kinds.length > 0 && (
                    <button type='button' className='application-sync-panel__kind application-sync-panel__kind--clear' onClick={() => onKindsChange([])}>
                        clear filter
                    </button>
                )}
            </div>
            {hiddenSelected > 0 && (
                <div className='application-details__warning'>
                    WARNING: {hiddenSelected} selected resource{hiddenSelected === 1 ? '' : 's'} hidden by the filter will also sync.{' '}
                    <button
                        type='button'
                        className='application-sync-panel__kind application-sync-panel__kind--clear'
                        onClick={() => onSelectionChange(deselectHidden(resources, selection, kinds))}>
                        deselect them
                    </button>
                </div>
            )}
        </div>
    );
};
