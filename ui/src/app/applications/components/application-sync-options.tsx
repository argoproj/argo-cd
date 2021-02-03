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

enum ManualSyncFlags {
    Prune = 'Prune',
    DryRun = 'Dry Run',
    ApplyOnly = 'Apply Only',
    Force = 'Force'
}

enum SyncOptions {
    Validate = 'Skip Schema Validation',
    CreateNamespace = 'Auto-Create Namespace',
    PruneLast = 'Prune Last',
    ApplyOutOfSyncOnly = 'Apply Out of Sync Only'
}

type SyncCheckboxKey = keyof typeof SyncOptions | keyof typeof ManualSyncFlags;

const optionStyle = {marginTop: '0.5em'};

export const ApplicationSyncOptions = (props: ApplicationSyncOptionProps) => (
    <React.Fragment>
        {Object.keys(SyncOptions).map(opt => (
            <div key={opt} style={optionStyle}>
                {booleanOption(opt, SyncOptions[opt as keyof typeof SyncOptions], false, props)}
            </div>
        ))}
    </React.Fragment>
);

export const ApplicationManualSyncFlags = (init: {[key: string]: boolean}, onChanged: (vals: {[key: string]: boolean}) => any) => {
    return (
        <React.Fragment>
            {Object.keys(ManualSyncFlags).map(flag => (
                <div key={flag} style={optionStyle}>
                    <Checkbox
                        id={`sync-option-${flag}`}
                        checked={init[flag]}
                        onChange={(val: boolean) => {
                            const changed = {...init};
                            changed[flag] = val;
                            onChanged(changed);
                        }}
                    />
                    <label htmlFor={`sync-option-${flag}`}>{ManualSyncFlags[flag as keyof typeof ManualSyncFlags]}</label>
                </div>
            ))}
        </React.Fragment>
    );
};

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

export const StringsToSyncOptions = (rawOpts: string[]): {[key: string]: boolean} => {
    const map: {[key: string]: boolean} = {};
    rawOpts.forEach(opt => {
        const split = opt.split('=');
        const key = split[0] as SyncCheckboxKey;
        const value = split[1] === 'true';
        return (map[key] = value);
    });
    return map;
};
