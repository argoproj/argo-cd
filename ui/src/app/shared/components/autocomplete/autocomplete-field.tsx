import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactForm from 'react-form';

import { Autocomplete, AutocompleteOption, AutocompleteProps } from './autocomplete';

export const AutocompleteField = ReactForm.FormField((props: AutocompleteProps & { fieldApi: ReactForm.FieldApi, className?: string }) => {
    const { fieldApi: {getValue, setValue, setTouched}, ...rest } = props;
    const value = getValue();

    const [forceHasValue, setForceHasValue] = React.useState(false);

    return (
        <Autocomplete
            wrapperProps={{className: classNames(props.className, { 'argo-has-value': forceHasValue })}}
            onSelect={(_, item) => {
                setValue(item.value);
            }}
            inputProps={{
                className: props.className,
                style: { borderBottom: 'none'},
            }}
            value={value}
            renderInput={(inputProps) => (
                <input
                    {...inputProps}
                    onFocus={(e) => {
                        if (inputProps.onFocus) {
                            inputProps.onFocus(e);
                        }
                        setForceHasValue(true);
                    }}
                    onBlur={(e) => {
                        if (inputProps.onBlur)  {
                            inputProps.onBlur(e);
                        }
                        setForceHasValue(false);
                        setTouched(true);
                    }}/>
            )}
            onChange={(val) => setValue(val.target.value)}
            {...rest}/>
    );
}) as React.ComponentType<ReactForm.FieldProps & { items: (AutocompleteOption | string)[], className?: string, filterSuggestions?: boolean }>;
