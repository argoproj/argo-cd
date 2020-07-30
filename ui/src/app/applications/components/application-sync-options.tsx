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

export const ApplicationSyncOptions = (props: ApplicationSyncOptionProps) => (
    <React.Fragment>
        <div style={optionStyle}>{booleanOption('Validate', 'Use a schema to validate resource manifests', true, props)}</div>
        <div style={optionStyle}>{booleanOption('CreateNamespace', 'Auto-create namespace', false, props)}</div>
    </React.Fragment>
);

export const ApplicationSyncOptionsField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue, setTouched}
    } = props;
    const val = getValue() || [];
    return (
        <div className='argo-field'>
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
