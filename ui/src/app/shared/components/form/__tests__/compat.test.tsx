import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {act} from 'react';
import {Form, FormApi, Text, TextArea, FormFieldHOC as FormField, ReactForm} from 'argo-ui';
const {Checkbox} = ReactForm;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Renders a Form and returns the captured FormApi via getApi. */
function renderForm(props: Partial<Parameters<typeof Form>[0]> & {children?: (api: FormApi) => React.ReactNode} = {}) {
    let capturedApi: FormApi | null = null;
    const {children, ...rest} = props;
    let instance: renderer.ReactTestRenderer;
    act(() => {
        instance = renderer.create(
            <Form
                defaultValues={rest.defaultValues || {}}
                getApi={api => { capturedApi = api; }}
                {...rest}
            >
                {children || ((_api: FormApi) => <div />)}
            </Form>
        );
    });
    return {api: capturedApi as unknown as FormApi, getInstance: () => instance};
}

// ===========================================================================
// 1a. Basic Form Rendering & Value Binding
// ===========================================================================

describe('Form – basic rendering & value binding', () => {
    test('renders without crashing', () => {
        const {getInstance} = renderForm({defaultValues: {name: 'hello'}});
        expect(getInstance().toJSON()).not.toBeNull();
    });

    test('getApi is called with a FormApi after mount', () => {
        const {api} = renderForm({defaultValues: {name: 'hello'}});
        expect(api).not.toBeNull();
        expect(typeof api.setValue).toBe('function');
    });

    test('formApi.values reflects defaultValues', () => {
        const {api} = renderForm({defaultValues: {name: 'alice', age: 30}});
        expect(api.values.name).toBe('alice');
        expect(api.values.age).toBe(30);
    });

    test('setValue updates values and getValue reflects new value', () => {
        const {api} = renderForm({defaultValues: {name: 'alice'}});
        act(() => { api.setValue('name', 'bob'); });
        expect(api.values.name).toBe('bob');
    });

    test('formDidUpdate fires after field change', () => {
        const formDidUpdate = jest.fn();
        const {api} = renderForm({
            defaultValues: {x: 1},
            formDidUpdate,
        });
        act(() => { api.setValue('x', 2); });
        expect(formDidUpdate).toHaveBeenCalled();
        const lastCall = formDidUpdate.mock.calls[formDidUpdate.mock.calls.length - 1][0];
        expect(lastCall.values.x).toBe(2);
    });
});

// ===========================================================================
// 1b. Deep-Path Field Access
// ===========================================================================

describe('Form – deep-path field access', () => {
    test('deepGet resolves nested path from defaultValues', () => {
        const {api} = renderForm({
            defaultValues: {spec: {source: {repoURL: 'https://github.com/example'}}},
        });
        expect(api.values.spec.source.repoURL).toBe('https://github.com/example');
    });

    test('setValue on nested path updates only the target key', () => {
        const {api} = renderForm({
            defaultValues: {spec: {source: {repoURL: 'old', path: '/app'}}},
        });
        act(() => { api.setValue('spec.source.repoURL', 'new'); });
        expect(api.values.spec.source.repoURL).toBe('new');
        expect(api.values.spec.source.path).toBe('/app'); // sibling preserved
    });

    test('setValue creates intermediate objects for non-existent path', () => {
        const {api} = renderForm({defaultValues: {}});
        act(() => { api.setValue('a.b.c', 'deep'); });
        expect(api.values.a.b.c).toBe('deep');
    });

    test('setValue on array-index-like path stores value under string key', () => {
        const {api} = renderForm({defaultValues: {items: {}}});
        act(() => { api.setValue('items.0.name', 'first'); });
        // String-keyed — not an actual array, but should not throw
        expect((api.values.items as any)['0'].name).toBe('first');
    });
});

// ===========================================================================
// 1c. Touch State & Validation
// ===========================================================================

describe('Form – touch state & validation', () => {
    test('setTouched marks a field as touched', () => {
        const {api} = renderForm({defaultValues: {email: ''}});
        act(() => { api.setTouched('email', true); });
        expect(api.touched.email).toBe(true);
    });

    test('setTouched can untouched a field', () => {
        const {api} = renderForm({defaultValues: {email: ''}});
        act(() => { api.setTouched('email', true); });
        act(() => { api.setTouched('email', false); });
        expect(api.touched.email).toBe(false);
    });

    test('submitForm calls validateError and sets errors when invalid', () => {
        const onSubmit = jest.fn();
        const onSubmitFailure = jest.fn();
        const {api} = renderForm({
            defaultValues: {name: ''},
            validateError: (vals) => ({name: vals.name === '' ? 'Required' : null}),
            onSubmit,
            onSubmitFailure,
        });
        act(() => { api.submitForm(null); });
        expect(api.errors.name).toBe('Required');
        expect(onSubmit).not.toHaveBeenCalled();
        expect(onSubmitFailure).toHaveBeenCalledWith({name: 'Required'});
    });

    test('submitForm calls onSubmit when validation passes', () => {
        const onSubmit = jest.fn();
        const {api} = renderForm({
            defaultValues: {name: 'alice'},
            validateError: (vals) => ({name: vals.name === '' ? 'Required' : null}),
            onSubmit,
        });
        act(() => { api.submitForm(null); });
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({name: 'alice'}), null, expect.anything());
    });

    test('setError sets a single field error', () => {
        const {api} = renderForm({defaultValues: {url: ''}});
        act(() => { api.setError('url', 'Invalid URL'); });
        expect(api.errors.url).toBe('Invalid URL');
    });
});

// ===========================================================================
// 1d. Form Submission
// ===========================================================================

describe('Form – submission', () => {
    test('submitForm calls onSubmit with current values', () => {
        const onSubmit = jest.fn();
        const {api} = renderForm({defaultValues: {count: 0}, onSubmit});
        act(() => { api.setValue('count', 5); });
        act(() => { api.submitForm(null); });
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({count: 5}), null, expect.anything());
    });

    test('submitForm calls e.preventDefault if event is provided', () => {
        const {api} = renderForm({defaultValues: {}, onSubmit: jest.fn()});
        const fakeEvent = {preventDefault: jest.fn()};
        act(() => { api.submitForm(fakeEvent); });
        expect(fakeEvent.preventDefault).toHaveBeenCalled();
    });

    test('preSubmit transforms values before onSubmit', () => {
        const onSubmit = jest.fn();
        const {api} = renderForm({
            defaultValues: {name: 'alice'},
            preSubmit: (vals) => ({...vals, name: vals.name.toUpperCase()}),
            onSubmit,
        });
        act(() => { api.submitForm(null); });
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({name: 'ALICE'}), null, expect.anything());
    });

    test('resetAll restores defaultValues and clears errors/touched', () => {
        const {api} = renderForm({defaultValues: {x: 'original'}});
        act(() => {
            api.setValue('x', 'changed');
            api.setTouched('x', true);
            api.setError('x', 'bad');
        });
        act(() => { api.resetAll(); });
        expect(api.values.x).toBe('original');
        expect(api.touched.x).toBeUndefined();
        expect(api.errors.x).toBeUndefined();
    });
});

// ===========================================================================
// 1e. Stale Closure & Concurrent Update Risks
// ===========================================================================

describe('Form – stale closure & ref consistency', () => {
    test('rapid sequential setValue calls all take effect', () => {
        const {api} = renderForm({defaultValues: {a: 0, b: 0, c: 0}});
        act(() => {
            api.setValue('a', 1);
            api.setValue('b', 2);
            api.setValue('c', 3);
        });
        expect(api.values.a).toBe(1);
        expect(api.values.b).toBe(2);
        expect(api.values.c).toBe(3);
    });

    test('getFormState returns consistent snapshot of all state buckets', () => {
        const {api} = renderForm({defaultValues: {x: 'hello'}});
        act(() => {
            api.setValue('x', 'world');
            api.setTouched('x', true);
            api.setError('x', 'oops');
        });
        const state = api.getFormState();
        expect(state.values.x).toBe('world');
        expect(state.touched.x).toBe(true);
        expect(state.errors.x).toBe('oops');
    });

    test('setAllValues replaces entire values object', () => {
        const {api} = renderForm({defaultValues: {a: 1, b: 2}});
        act(() => { api.setAllValues({a: 10, b: 20, c: 30}); });
        expect(api.values).toEqual({a: 10, b: 20, c: 30});
    });

    test('setFormState can update values, touched, and errors simultaneously', () => {
        const {api} = renderForm({defaultValues: {n: 0}});
        act(() => {
            api.setFormState({
                values: {n: 99},
                touched: {n: true},
                errors: {n: 'too large'},
            });
        });
        expect(api.values.n).toBe(99);
        expect(api.touched.n).toBe(true);
        expect(api.errors.n).toBe('too large');
    });
});

// ===========================================================================
// 1f. withFieldApi HOC / FormField
// ===========================================================================

describe('withFieldApi / FormField HOC', () => {
    test('wrapped component reads value from FormContext', () => {
        let capturedValue: any;
        const Inspector = FormField(({fieldApi}: any) => {
            capturedValue = fieldApi.getValue();
            return null;
        });

        act(() => {
            renderer.create(
                <Form defaultValues={{color: 'blue'}}>
                    {() => <Inspector field='color' />}
                </Form>
            );
        });
        expect(capturedValue).toBe('blue');
    });

    test('wrapped component writes value through FormContext', () => {
        let capturedApi: FormApi | null = null;
        const Writer = FormField(({fieldApi}: any) => {
            React.useEffect(() => { fieldApi.setValue('green'); }, []);
            return null;
        });

        act(() => {
            renderer.create(
                <Form defaultValues={{color: 'blue'}} getApi={api => { capturedApi = api; }}>
                    {() => <Writer field='color' />}
                </Form>
            );
        });
        expect(capturedApi!.values.color).toBe('green');
    });

    test('wrapped component works with explicit formApi prop outside Form', () => {
        let capturedValue: any;
        const fakeFormApi: FormApi = {
            values: {token: 'abc'},
            touched: {},
            errors: {},
            submitForm: jest.fn(),
            setError: jest.fn(),
            getFormState: jest.fn(),
            setFormState: jest.fn(),
            setAllValues: jest.fn(),
            setValue: jest.fn(),
            setTouched: jest.fn(),
            resetAll: jest.fn(),
        };

        const Inspector = FormField(({fieldApi}: any) => {
            capturedValue = fieldApi.getValue();
            return null;
        });

        act(() => {
            renderer.create(<Inspector field='token' formApi={fakeFormApi} />);
        });
        expect(capturedValue).toBe('abc');
    });

    test('throws readable error when used outside Form without formApi', () => {
        const BadComponent = FormField(() => null);
        expect(() => {
            act(() => {
                renderer.create(<BadComponent field='x' />);
            });
        }).toThrow('FormField components must be used inside <Form> or be passed formApi');
    });
});

// ===========================================================================
// 1g. Pre-built field components
// ===========================================================================

describe('Text field component', () => {
    test('renders an input with the current field value', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{name: 'alice'}}>
                    {() => <Text field='name' />}
                </Form>
            );
        });
        const input = tree!.root.findByType('input');
        expect(input.props.value).toBe('alice');
    });

    test('onChange updates form value', () => {
        let capturedApi: FormApi | null = null;
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{name: 'alice'}} getApi={api => { capturedApi = api; }}>
                    {() => <Text field='name' />}
                </Form>
            );
        });
        const input = tree!.root.findByType('input');
        act(() => {
            input.props.onChange({currentTarget: {value: 'bob'}});
        });
        expect(capturedApi!.values.name).toBe('bob');
    });

    test('onBlur marks field as touched', () => {
        let capturedApi: FormApi | null = null;
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{name: ''}} getApi={api => { capturedApi = api; }}>
                    {() => <Text field='name' />}
                </Form>
            );
        });
        const input = tree!.root.findByType('input');
        act(() => {
            input.props.onBlur({});
        });
        expect(capturedApi!.touched.name).toBe(true);
    });

    test('renders empty string for undefined field value', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{}}>
                    {() => <Text field='missing' />}
                </Form>
            );
        });
        const input = tree!.root.findByType('input');
        expect(input.props.value).toBe('');
    });
});

describe('TextArea field component', () => {
    test('renders a textarea with the current field value', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{notes: 'hello'}}>
                    {() => <TextArea field='notes' />}
                </Form>
            );
        });
        const ta = tree!.root.findByType('textarea');
        expect(ta.props.value).toBe('hello');
    });

    test('onChange updates form value', () => {
        let capturedApi: FormApi | null = null;
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{notes: 'hello'}} getApi={api => { capturedApi = api; }}>
                    {() => <TextArea field='notes' />}
                </Form>
            );
        });
        const ta = tree!.root.findByType('textarea');
        act(() => { ta.props.onChange({currentTarget: {value: 'world'}}); });
        expect(capturedApi!.values.notes).toBe('world');
    });
});

describe('Checkbox field component', () => {
    test('renders a checkbox with correct checked state', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{enabled: true}}>
                    {() => <Checkbox field='enabled' />}
                </Form>
            );
        });
        const cb = tree!.root.findByType('input');
        expect(cb.props.type).toBe('checkbox');
        expect(cb.props.checked).toBe(true);
    });

    test('onChange toggles boolean value', () => {
        let capturedApi: FormApi | null = null;
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{enabled: true}} getApi={api => { capturedApi = api; }}>
                    {() => <Checkbox field='enabled' />}
                </Form>
            );
        });
        const cb = tree!.root.findByType('input');
        act(() => { cb.props.onChange({currentTarget: {checked: false}}); });
        expect(capturedApi!.values.enabled).toBe(false);
    });

    test('treats falsy undefined field as unchecked', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <Form defaultValues={{}}>
                    {() => <Checkbox field='missing' />}
                </Form>
            );
        });
        const cb = tree!.root.findByType('input');
        expect(cb.props.checked).toBe(false);
    });
});
