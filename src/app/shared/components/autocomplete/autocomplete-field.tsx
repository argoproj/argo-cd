import * as React from 'react';
import * as ReactForm from 'react-form';
import { Autocomplete, AutocompleteOption, AutocompleteProps } from './autocomplete';

export const AutocompleteField = ReactForm.FormField((props: AutocompleteProps & { fieldApi: ReactForm.FieldApi, className?: string }) => {
    const { fieldApi: {getValue, setValue}, ...rest } = props;
    const value = getValue();

    return (
        <Autocomplete
            wrapperProps={{className: props.className}}
            onSelect={(selected) => {
                setValue(selected);
            }}
            inputProps={{
                className: props.className,
                style: { borderBottom: 'none',
            }}}
            value={value}
            onChange={(val) => setValue(val.target.value)}
            {...rest}/>
    );
}) as React.ComponentType<ReactForm.FieldProps & { items: (AutocompleteOption | string)[], className?: string }>;
