import * as React from 'react';
import {render, screen, fireEvent} from '@testing-library/react';
import {act} from 'react';
import {
    ArrayInput,
    NameValueEditor,
    ValueEditor,
    ArrayInputField,
    MapInputField,
    ArrayValueField,
    StringValueField,
    MapValueField,
    ResetOrDeleteButton,
    NameValue,
} from '../array-input';
import {Form, FormApi} from 'argo-ui';

// ---------------------------------------------------------------------------
// Helper: render an ArrayInput field inside a Form and return the FormApi
// ---------------------------------------------------------------------------

function renderWithForm<T>(
    FieldComponent: React.ComponentType<any>,
    fieldProps: Record<string, any>,
    defaultValues: Record<string, any>
): {api: FormApi} {
    let capturedApi: FormApi | null = null;
    render(
        <Form defaultValues={defaultValues} getApi={api => { capturedApi = api; }}>
            {() => <FieldComponent field='items' {...fieldProps} />}
        </Form>
    );
    return {api: capturedApi as unknown as FormApi};
}

// ===========================================================================
// 3a. Basic Array Operations — ArrayInput<T>
// ===========================================================================

describe('ArrayInput – basic operations', () => {
    test('renders all items', () => {
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        render(<ArrayInput items={items} onChange={jest.fn()} editor={NameValueEditor} />);
        expect(screen.queryByDisplayValue('A')).toBeTruthy();
        expect(screen.queryByDisplayValue('B')).toBeTruthy();
    });

    test('renders "No items" label when empty', () => {
        render(<ArrayInput items={[]} onChange={jest.fn()} editor={NameValueEditor} />);
        expect(screen.getByText('No items')).toBeTruthy();
    });

    test('clicking the + button calls onChange with a new empty item appended', () => {
        const onChange = jest.fn();
        const items: NameValue[] = [{name: 'X', value: '1'}];
        render(<ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />);

        const addButton = document.querySelector('button .fa-plus')!.closest('button') as HTMLElement;
        act(() => { fireEvent.click(addButton); });

        expect(onChange).toHaveBeenCalledTimes(1);
        const newItems = onChange.mock.calls[0][0];
        expect(newItems).toHaveLength(2);
        expect(newItems[0]).toEqual({name: 'X', value: '1'}); // original preserved
        expect(newItems[1]).toEqual({});                        // new empty item
    });

    test('clicking × on an item removes it and preserves the rest', () => {
        const onChange = jest.fn();
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}, {name: 'C', value: '3'}];
        render(<ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />);

        // Find all × icons (fa-times) — onClick is on the <i> element
        const removeIcons = document.querySelectorAll('i.fa-times');
        // Remove the second item (index 1)
        act(() => { fireEvent.click(removeIcons[1]); });

        expect(onChange).toHaveBeenCalledTimes(1);
        const newItems = onChange.mock.calls[0][0];
        expect(newItems).toHaveLength(2);
        expect(newItems[0]).toEqual({name: 'A', value: '1'});
        expect(newItems[1]).toEqual({name: 'C', value: '3'});
    });

    test('editing an item calls onChange with the updated item in place', () => {
        const onChange = jest.fn();
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        render(<ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />);

        // The first name input (title='Name')
        const inputs = screen.getAllByTitle('Name');
        act(() => { fireEvent.change(inputs[0], {target: {value: 'Z'}}); });

        const newItems = onChange.mock.calls[0][0];
        expect(newItems[0].name).toBe('Z');
        expect(newItems[1]).toEqual({name: 'B', value: '2'}); // untouched
    });
});

// ===========================================================================
// 3b. Key Stability
// ===========================================================================

describe('ArrayInput – key stability', () => {
    test('uses index-based keys (documents known limitation)', () => {
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        render(<ArrayInput items={items} onChange={jest.fn()} editor={NameValueEditor} />);
        // Each item renders an × icon — one per item
        const removeIcons = document.querySelectorAll('i.fa-times');
        expect(removeIcons).toHaveLength(2);
    });

    test('removing the first item does not lose data from the second item', () => {
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        const captured: NameValue[][] = [];
        const onChange = (newItems: NameValue[]) => captured.push(newItems);
        render(<ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />);

        const removeIcons = document.querySelectorAll('i.fa-times');
        act(() => { fireEvent.click(removeIcons[0]); });

        const remaining = captured[0];
        expect(remaining).toHaveLength(1);
        expect(remaining[0]).toEqual({name: 'B', value: '2'});
    });
});

// ===========================================================================
// 3c. ArrayInputField (form-connected)
// ===========================================================================

describe('ArrayInputField – form connected', () => {
    test('renders items from form values', () => {
        renderWithForm(ArrayInputField, {}, {
            items: [{name: 'FOO', value: 'bar'}],
        });
        expect(screen.queryByDisplayValue('FOO')).toBeTruthy();
        expect(screen.queryByDisplayValue('bar')).toBeTruthy();
    });

    test('adding an item updates form value', () => {
        const {api} = renderWithForm(ArrayInputField, {}, {
            items: [{name: 'A', value: '1'}],
        });

        const addButton = document.querySelector('button .fa-plus')!.closest('button') as HTMLElement;
        act(() => { fireEvent.click(addButton); });

        expect(api.values.items).toHaveLength(2);
    });

    test('removing an item updates form value', () => {
        const {api} = renderWithForm(ArrayInputField, {}, {
            items: [{name: 'A', value: '1'}, {name: 'B', value: '2'}],
        });

        const removeIcons = document.querySelectorAll('i.fa-times');
        act(() => { fireEvent.click(removeIcons[0]); });

        expect(api.values.items).toHaveLength(1);
        expect(api.values.items[0].name).toBe('B');
    });
});

// ===========================================================================
// 3d. MapInputField – object ↔ name-value-pair conversion
// ===========================================================================

describe('MapInputField – object conversion', () => {
    test('renders object as name-value pairs', () => {
        renderWithForm(MapInputField, {}, {
            items: {foo: 'bar', baz: 'qux'},
        });
        expect(screen.queryByDisplayValue('foo')).toBeTruthy();
        expect(screen.queryByDisplayValue('bar')).toBeTruthy();
    });

    test('editing a pair updates the object key/value', () => {
        const {api} = renderWithForm(MapInputField, {}, {
            items: {foo: 'bar'},
        });

        // Edit the value input (title='Value')
        const valueInputs = screen.getAllByTitle('Value');
        act(() => { fireEvent.change(valueInputs[0], {target: {value: 'newval'}}); });

        expect(api.values.items.foo).toBe('newval');
    });

    test('adding a pair appends to the object', () => {
        const {api} = renderWithForm(MapInputField, {}, {
            items: {existing: 'value'},
        });

        const addButton = document.querySelector('button .fa-plus')!.closest('button') as HTMLElement;
        act(() => { fireEvent.click(addButton); });

        // New empty key '' should be present
        expect(Object.keys(api.values.items)).toHaveLength(2);
    });

    test('removing a pair removes the key from the object', () => {
        const {api} = renderWithForm(MapInputField, {}, {
            items: {a: '1', b: '2'},
        });

        const removeIcons = document.querySelectorAll('i.fa-times');
        act(() => { fireEvent.click(removeIcons[0]); });

        expect(Object.keys(api.values.items)).toHaveLength(1);
    });
});

// ===========================================================================
// 3e. ResetOrDeleteButton
// ===========================================================================

describe('ResetOrDeleteButton', () => {
    function makeProps(overrides: Partial<React.ComponentProps<typeof ResetOrDeleteButton>> = {}) {
        return {
            isPluginPar: false,
            getValue: jest.fn().mockReturnValue([{name: 'myParam', string: 'val'}]),
            name: 'myParam',
            index: 0,
            setValue: jest.fn(),
            setAppParamsDeletedState: jest.fn(),
            ...overrides,
        };
    }

    test('renders Delete button for non-plugin parameter', () => {
        render(<ResetOrDeleteButton {...makeProps()} />);
        expect(screen.getByRole('button', {name: /Delete/i})).toBeTruthy();
    });

    test('renders Reset button for plugin parameter', () => {
        render(<ResetOrDeleteButton {...makeProps({isPluginPar: true})} />);
        expect(screen.getByRole('button', {name: /Reset/i})).toBeTruthy();
    });

    test('Delete button calls setAppParamsDeletedState with parameter name', () => {
        const props = makeProps();
        render(<ResetOrDeleteButton {...props} />);
        const btn = screen.getByRole('button');
        act(() => { fireEvent.click(btn); });
        expect(props.setAppParamsDeletedState).toHaveBeenCalledTimes(1);
    });

    test('Reset button removes item from array via setValue', () => {
        const items = [{name: 'p1', string: 'v1'}, {name: 'p2', string: 'v2'}];
        const setValue = jest.fn();
        const props = makeProps({
            isPluginPar: true,
            getValue: jest.fn().mockReturnValue(items),
            name: 'p1',
            index: 0,
            setValue,
        });
        render(<ResetOrDeleteButton {...props} />);
        const btn = screen.getByRole('button');
        act(() => { fireEvent.click(btn); });
        expect(setValue).toHaveBeenCalledTimes(1);
        const newVal = setValue.mock.calls[0][0];
        expect(newVal).toHaveLength(1);
        expect(newVal[0].name).toBe('p2');
    });

    test('button is disabled when index is -1', () => {
        render(<ResetOrDeleteButton {...makeProps({index: -1})} />);
        const btn = screen.getByRole('button') as HTMLButtonElement;
        expect(btn.disabled).toBe(true);
    });

    test('does not call handlers when index is -1', () => {
        const props = makeProps({index: -1});
        render(<ResetOrDeleteButton {...props} />);
        const btn = screen.getByRole('button');
        act(() => { fireEvent.click(btn); });
        // Neither handler should be called for non-plugin par when index === -1
        // (button is disabled so click is a no-op in a real browser, but we test the handlers)
        expect(props.setAppParamsDeletedState).not.toHaveBeenCalled();
    });
});

// ===========================================================================
// 3f. ReadOnly Mode — NameValueEditor / ValueEditor
// ===========================================================================

describe('NameValueEditor – readonly mode', () => {
    test('inputs are readOnly when onChange is not provided', () => {
        render(<>{NameValueEditor({name: 'foo', value: 'bar'})}</>);
        const inputs = screen.getAllByRole('textbox') as HTMLInputElement[];
        inputs.forEach(input => {
            expect(input.readOnly).toBe(true);
        });
    });

    test('inputs are editable when onChange is provided', () => {
        render(<>{NameValueEditor({name: 'foo', value: 'bar'}, jest.fn())}</>);
        const inputs = screen.getAllByRole('textbox') as HTMLInputElement[];
        inputs.forEach(input => {
            expect(input.readOnly).toBe(false);
        });
    });
});

describe('ValueEditor – readonly mode', () => {
    test('input is readOnly when onChange is falsy', () => {
        render(<>{ValueEditor('hello', null as any)}</>);
        const input = screen.getByRole('textbox') as HTMLInputElement;
        expect(input.readOnly).toBe(true);
    });
});

// ===========================================================================
// 3g. ArrayValueField — direct mutation risk
// ===========================================================================

describe('ArrayValueField – rapid updates', () => {
    function renderArrayValueField(defaultItems: any[]) {
        let capturedApi: FormApi | null = null;
        render(
            <Form defaultValues={{items: defaultItems}} getApi={api => { capturedApi = api; }}>
                {() => (
                    <ArrayValueField
                        field='items'
                        name='myParam'
                        defaultVal={['default']}
                        isPluginPar={true}
                        setAppParamsDeletedState={jest.fn()}
                    />
                )}
            </Form>
        );
        return {api: capturedApi as unknown as FormApi};
    }

    test('renders array values for a named parameter', () => {
        renderArrayValueField([{name: 'myParam', array: ['val1', 'val2']}]);
        expect(screen.queryByDisplayValue('val1')).toBeTruthy();
        expect(screen.queryByDisplayValue('val2')).toBeTruthy();
    });

    test('shows default values when parameter not yet overridden', () => {
        renderArrayValueField([]);
        expect(screen.queryByDisplayValue('default')).toBeTruthy();
    });

    test('adding a value appends to the parameter array', () => {
        const {api} = renderArrayValueField([{name: 'myParam', array: ['a']}]);
        const addButton = document.querySelector('button .fa-plus')!.closest('button') as HTMLElement;
        act(() => { fireEvent.click(addButton); });
        const param = api.values.items.find((p: any) => p.name === 'myParam');
        expect(param.array).toHaveLength(2);
    });
});

// ===========================================================================
// 3h. StringValueField
// ===========================================================================

describe('StringValueField', () => {
    function renderStringValueField(defaultItems: any[]) {
        let capturedApi: FormApi | null = null;
        render(
            <Form defaultValues={{items: defaultItems}} getApi={api => { capturedApi = api; }}>
                {() => (
                    <StringValueField
                        field='items'
                        name='myParam'
                        defaultVal='default_string'
                        isPluginPar={false}
                        setAppParamsDeletedState={jest.fn()}
                    />
                )}
            </Form>
        );
        return {api: capturedApi as unknown as FormApi};
    }

    test('renders default value when parameter not overridden', () => {
        renderStringValueField([]);
        expect(screen.queryByDisplayValue('default_string')).toBeTruthy();
    });

    test('typing a value creates/updates the parameter', () => {
        const {api} = renderStringValueField([]);
        const input = screen.getByPlaceholderText('Value');
        act(() => { fireEvent.change(input, {target: {value: 'typed_value'}}); });
        const param = api.values.items.find((p: any) => p.name === 'myParam');
        expect(param.string).toBe('typed_value');
    });

    test('updating an existing parameter string value', () => {
        const {api} = renderStringValueField([{name: 'myParam', string: 'initial'}]);
        const input = screen.getByPlaceholderText('Value');
        act(() => { fireEvent.change(input, {target: {value: 'updated'}}); });
        const param = api.values.items.find((p: any) => p.name === 'myParam');
        expect(param.string).toBe('updated');
    });
});
