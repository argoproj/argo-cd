import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';
import {NumberField} from './number-field';

describe('NumberField', () => {
    let formApi: FormApi;

    beforeEach(() => {
        render(
            <Form defaultValues={{value: 123}} getApi={api => (formApi = api)}>
                {() => <NumberField field='value' />}
            </Form>
        );
    });

    test('sets undefined when cleared', () => {
        fireEvent.change(screen.getByRole('spinbutton'), {target: {value: ''}});
        expect(formApi.values.value).toBeUndefined();
    });

    test('preserves zero as a numeric value', () => {
        fireEvent.change(screen.getByRole('spinbutton'), {target: {value: '0'}});
        expect(formApi.values.value).toBe(0);
    });
});
