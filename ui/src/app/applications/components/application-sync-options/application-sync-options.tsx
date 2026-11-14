import {Checkbox, Select, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import * as React from 'react';
import {useQuery} from '@tanstack/react-query';
import {ReactForm} from 'argo-ui';

import './application-sync-options.scss';
import {settingsQueries} from '../../../shared/queries';
import {loading, loadFailed} from '../../../shared/query-client';

const ReplaceWarning = () => (
    <div>
        <p>
            Argo CD will sync using <strong>kubectl replace/create</strong>. This operation <strong>forces resource deletion and recreation</strong>. Proceed only if you understand
            the risks.
        </p>
    </div>
);

const PruneAllWarning = () => (
    <div>
        <p>
            The resources will be synced using --prune, and all resources are marked to be pruned. Only continue if you want to{' '}
            <strong>delete all of the Application's resources.</strong>
        </p>
    </div>
);

const PruneSomeWarning = () => (
    <div>
        <p>
            The resources will be synced using --prune, and some resources are marked to be pruned. Only continue if you want to <strong>delete the pruned resources</strong>.
        </p>
    </div>
);

export const REPLACE_WARNING = <ReplaceWarning />;
export const FORCE_WARNING = `The resources will be synced using '--force' that is a potentially destructive action and will immediately remove resources from the API and bypasses graceful deletion. Immediate deletion of some resources may result in inconsistency or data loss.`;
export const PRUNE_ALL_WARNING = <PruneAllWarning />;
export const PRUNE_SOME_WARNING = <PruneSomeWarning />;

export interface ApplicationSyncOptionProps {
    options: string[];
    onChanged: (updatedOptions: string[]) => any;
    id?: string;
}

function selectOption(name: string, label: string, defaultVal: string, values: string[], props: ApplicationSyncOptionProps) {
    const options = [...(props.options || [])];
    const prefix = `${name}=`;
    const index = options.findIndex(item => item.startsWith(prefix));
    const val = index < 0 ? defaultVal : options[index].substr(prefix.length);

    return (
        <div className='application-sync-options__select'>
            <label>{label}:</label>
            <Select
                value={val}
                options={values}
                onChange={opt => {
                    const newValue = `${name}=${opt.value}`;
                    if (index < 0) {
                        props.onChanged(options.concat(newValue));
                    } else {
                        options[index] = newValue;
                        props.onChanged(options);
                    }
                }}
            />
        </div>
    );
}

function booleanOption(name: string, label: string, defaultVal: boolean, props: ApplicationSyncOptionProps, invert: boolean, warning: string | React.ReactNode = null) {
    const options = [...(props.options || [])];
    const prefix = `${name}=`;
    const index = options.findIndex(item => item.startsWith(prefix));
    const checked = index < 0 ? defaultVal : options[index].substring(prefix.length) === (invert ? 'false' : 'true');
    return (
        <React.Fragment>
            <Checkbox
                id={`sync-option-${name}-${props.id}`}
                checked={checked}
                onChange={(val: boolean) => {
                    if (index < 0) {
                        props.onChanged(options.concat(`${name}=${invert ? !val : val}`));
                    } else {
                        options.splice(index, 1);
                        props.onChanged(options);
                    }
                }}
            />
            <label htmlFor={`sync-option-${name}-${props.id}`}>{label}</label>{' '}
            {warning && (
                <>
                    <Tooltip content={typeof warning === 'string' ? warning : 'Warning'}>
                        <i className='fa fa-exclamation-triangle' />
                    </Tooltip>
                    {checked && <div className='application-sync-options__warning'>{warning}</div>}
                </>
            )}
        </React.Fragment>
    );
}

enum ManualSyncFlags {
    Prune = 'Prune',
    DryRun = 'Dry Run',
    ApplyOnly = 'Apply Only',
    Force = 'Force'
}

export interface SyncFlags {
    Prune: boolean;
    DryRun: boolean;
    ApplyOnly: boolean;
    Force: boolean;
}

const syncOptions: Array<(props: ApplicationSyncOptionProps) => React.ReactNode> = [
    props => booleanOption('Validate', 'Skip Schema Validation', false, props, true),
    props => booleanOption('CreateNamespace', 'Auto-Create Namespace', false, props, false),
    props => booleanOption('ApplyOutOfSyncOnly', 'Apply Out of Sync Only', false, props, false),
    props => booleanOption('RespectIgnoreDifferences', 'Respect Ignore Differences', false, props, false),
    props => booleanOption('ServerSideApply', 'Server-Side Apply', false, props, false),
    props => booleanOption('PruneLast', 'Prune Last', false, props, false),
    props => selectOption('PrunePropagationPolicy', 'Prune Propagation Policy', 'foreground', ['foreground', 'background', 'orphan'], props),
    props => selectOption('Prune', 'Prune', 'true', ['true', 'false', 'confirm'], props),
    props => selectOption('Delete', 'Delete', 'true', ['true', 'false', 'confirm'], props)
];

const optionStyle = {marginTop: '0.5em'};

// Replace is only offered when the server allows it (--sync-with-replace-allowed).
const ReplaceOption = (props: ApplicationSyncOptionProps) => {
    // refetchOnMount 'always': every mount refetches, whatever staleTime the query or client sets.
    // networkMode 'always': offline, the refetch fails (as DataLoader's did) instead of pausing with the cached value shown.
    const settings = useQuery({...settingsQueries.get(), networkMode: 'always', refetchOnMount: 'always'});
    // Decide on freshly loaded settings, as before: a cached value could still offer Replace after the server stopped allowing it.
    if (settings.isFetching || settings.isError || !settings.data) {
        return settings.isError && !settings.isFetching ? loadFailed(settings.refetch) : loading;
    }
    if (!settings.data.syncWithReplaceAllowed) {
        return null;
    }
    return (
        <div className='small-12' style={optionStyle}>
            {booleanOption('Replace', 'Replace', false, props, false)}
        </div>
    );
};

export const ApplicationSyncOptions = (props: ApplicationSyncOptionProps) => (
    <div className='row application-sync-options'>
        {syncOptions.map((render, i) => (
            <div
                key={i}
                style={optionStyle}
                className={classNames('small-12', {
                    'large-6': i < syncOptions.length - 3
                })}>
                {render(props)}
            </div>
        ))}
        <ReplaceOption {...props} />
    </div>
);

export const ApplicationManualSyncFlags = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi; id?: string}) => {
    const {
        fieldApi: {getValue, setValue, setTouched}
    } = props;
    const val = getValue() || false;
    return (
        <div style={optionStyle}>
            {Object.keys(ManualSyncFlags).map(flag => (
                <React.Fragment key={flag}>
                    <Checkbox
                        id={`sync-option-${flag}-${props.id}`}
                        checked={val[flag]}
                        onChange={(newVal: boolean) => {
                            setTouched(true);
                            const update = {...val};
                            update[flag] = newVal;
                            setValue(update);
                        }}
                    />
                    <label htmlFor={`sync-option-${flag}-${props.id}`}>{ManualSyncFlags[flag as keyof typeof ManualSyncFlags]}</label>{' '}
                </React.Fragment>
            ))}
        </div>
    );
});

export const ApplicationSyncOptionsField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue, setTouched}
    } = props;
    const val = getValue() || [];
    return (
        <div className='argo-field' style={{borderBottom: '0'}}>
            <ApplicationSyncOptions
                options={val}
                onChanged={opts => {
                    setTouched(true);
                    setValue(opts);
                }}
            />
        </div>
    );
});
