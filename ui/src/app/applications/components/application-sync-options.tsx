import {Checkbox} from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';

export interface ApplicationSyncOptionProps {
    options: string[];
    onChanged: (updatedOptions: string[]) => any;
}

function booleanOption(name: string, label: string, defaultVal: boolean, props: ApplicationSyncOptionProps) {
    const options = props.options.slice();
    const prefix = `${name}=`;
    const index = options.findIndex(item => item.startsWith(prefix));
    const checked = index < 0 ? defaultVal : options[index].substring(prefix.length) === 'true';
    return (
        <React.Fragment>
            <Checkbox
                id={`sync-option-${name}`}
                checked={checked}
                onChange={(val: boolean) => {
                    if (index < 0) {
                        props.onChanged(options.concat(`${name}=${val}`));
                    } else {
                        options.splice(index, 1);
                        props.onChanged(options);
                    }
                }}
            />
            <label htmlFor={`sync-option-${name}`}>{label}</label>
        </React.Fragment>
    );
}

const optionStyle: React.CSSProperties = {marginTop: '0.5em'};

interface SyncOption {
    name?: string;
    key: SyncOptionKey;
    value?: boolean;
    default?: boolean;
}

const enum SyncOptions {
    Validate = 'Validate',
    CreateNamespace = 'Auto-Create Namespace',
    Prune = 'Prune',
    DryRun = 'Dry Run',
    ApplyOnly = 'Apply Only',
    Force = 'Force'
}

type SyncOptionKey = keyof typeof SyncOptions;

const syncOptions: SyncOption[] = [
    {
        key: 'Validate',
        default: true
    },
    {
        name: 'Auto-Create Namespace',
        key: 'CreateNamespace'
    },
    {key: 'Prune'},
    {key: 'DryRun', name: 'Dry Run'},
    {key: 'ApplyOnly', name: 'Apply Only'},
    {key: 'Force'}
];

export const ApplicationSyncOptions = (props: ApplicationSyncOptionProps) => (
    <React.Fragment>
        {syncOptions.map(opt => (
            <div key={opt.key} style={optionStyle}>
                {booleanOption(opt.key, opt.name || opt.key, !!opt.default, props)}
            </div>
        ))}
    </React.Fragment>
);

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

export const StringsToSyncOptions = (rawOpts: string[]): {[key: string]: SyncOption} => {
    const map: {[key: string]: SyncOption} = {};
    rawOpts.forEach(opt => {
        const split = opt.split('=');
        const key = split[0] as SyncOptionKey;
        const value = split[1] === 'true';
        return (map[key] = {key, value});
    });
    return map;
};
