import {Checkbox, Select, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactForm from 'react-form';

require('./application-sync-options.scss');

export const REPLACE_WARNING = `The resources will be synced using 'kubectl replace/create' command that is a potentially destructive action and might cause resources recreation.`;
export const FORCE_WARNING = `The resources will be synced using '--force' that is a potentially destructive action and will immediately remove resources from the API and bypasses graceful deletion. Immediate deletion of some resources may result in inconsistency or data loss.`;

export interface ApplicationSyncOptionProps {
    options: string[];
    onChanged: (updatedOptions: string[]) => any;
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

function booleanOption(name: string, label: string, defaultVal: boolean, props: ApplicationSyncOptionProps, invert: boolean, warning: string = null) {
    const options = [...(props.options || [])];
    const prefix = `${name}=`;
    const index = options.findIndex(item => item.startsWith(prefix));
    const checked = index < 0 ? defaultVal : options[index].substring(prefix.length) === (invert ? 'false' : 'true');
    return (
        <React.Fragment>
            <Checkbox
                id={`sync-option-${name}`}
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
            <label htmlFor={`sync-option-${name}`}>{label}</label>{' '}
            {warning && (
                <>
                    <Tooltip content={warning}>
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
    props => booleanOption('PruneLast', 'Prune Last', false, props, false),
    props => booleanOption('ApplyOutOfSyncOnly', 'Apply Out of Sync Only', false, props, false),
    props => booleanOption('RespectIgnoreDifferences', 'Respect Ignore Differences', false, props, false),
    props => selectOption('PrunePropagationPolicy', 'Prune Propagation Policy', 'foreground', ['foreground', 'background', 'orphan'], props)
];

const optionStyle = {marginTop: '0.5em'};

export const ApplicationSyncOptions = (props: ApplicationSyncOptionProps) => (
    <div className='row application-sync-options'>
        {syncOptions.map((render, i) => (
            <div
                key={i}
                style={optionStyle}
                className={classNames('small-12', {
                    'large-6': i < syncOptions.length - 1
                })}>
                {render(props)}
            </div>
        ))}
        <div className='small-12' style={optionStyle}>
            {booleanOption('Replace', 'Replace', false, props, false, REPLACE_WARNING)}
        </div>
    </div>
);

export const ApplicationManualSyncFlags = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue, setTouched}
    } = props;
    const val = getValue() || false;
    return (
        <div style={optionStyle}>
            {Object.keys(ManualSyncFlags).map(flag => (
                <React.Fragment key={flag}>
                    <Checkbox
                        id={`sync-option-${flag}`}
                        checked={val[flag]}
                        onChange={(newVal: boolean) => {
                            setTouched(true);
                            const update = {...val};
                            update[flag] = newVal;
                            setValue(update);
                        }}
                    />
                    <label htmlFor={`sync-option-${flag}`}>{ManualSyncFlags[flag as keyof typeof ManualSyncFlags]}</label>{' '}
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
