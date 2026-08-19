import * as React from 'react';
import {render} from '@testing-library/react';
// Types only — jest.setup.js registers the matchers at runtime, but tsconfig's `types` pins jest+node.
import '@testing-library/jest-dom';
import {EntryField, EntryFieldList} from './entry-fields';

// The list views rely on this markup contract, so assert it directly:
// - <dt>/<dd> pairs are direct children of the <dl> (a <dl> requires it, and the table's
//   flat grid places them as individual grid items);
// - each cell carries `entry-fields__d{t,d}--${name}`, which is what the SCSS pins to a
//   grid cell and what hides the Status term;
// - the variant picks the reused, already-tuned tile/table cell classes.

const dl = (c: HTMLElement) => c.querySelector('dl');

test('renders each field as a dt/dd pair directly under the dl', () => {
    const {container} = render(
        <EntryFieldList variant='table'>
            <EntryField name='project' label='Project'>
                default
            </EntryField>
            <EntryField name='name' label='Name'>
                my-app
            </EntryField>
        </EntryFieldList>
    );

    const cells = Array.from(dl(container).children);
    expect(cells.map(el => el.tagName)).toEqual(['DT', 'DD', 'DT', 'DD']);
    expect(cells.map(el => el.textContent)).toEqual(['Project:', 'default', 'Name:', 'my-app']);
});

test('names each cell so the SCSS can place it', () => {
    const {container} = render(
        <EntryFieldList variant='table'>
            <EntryField name='status' label='Status'>
                Healthy
            </EntryField>
        </EntryFieldList>
    );

    expect(container.querySelector('dt')).toHaveClass('entry-fields__dt', 'entry-fields__dt--status');
    expect(container.querySelector('dd')).toHaveClass('entry-fields__dd', 'entry-fields__dd--status');
});

test.each([
    ['tile' as const, 'applications-tiles__field-label', 'applications-tiles__field-value'],
    ['table' as const, 'applications-list__meta-label', 'applications-list__meta-value']
])('the %s variant reuses that view’s cell classes', (variant, dtClass, ddClass) => {
    const {container} = render(
        <EntryFieldList variant={variant}>
            <EntryField name='project' label='Project'>
                default
            </EntryField>
        </EntryFieldList>
    );

    expect(dl(container)).toHaveClass('entry-fields', `entry-fields--${variant}`);
    expect(container.querySelector('dt')).toHaveClass(dtClass);
    expect(container.querySelector('dd')).toHaveClass(ddClass);
});

test('appends className to the dl and valueClassName to the dd', () => {
    const {container} = render(
        <EntryFieldList variant='table' className='applications-list__meta-flat'>
            <EntryField name='source' label='Source' valueClassName='applications-table-source'>
                repo
            </EntryField>
        </EntryFieldList>
    );

    expect(dl(container)).toHaveClass('entry-fields--table', 'applications-list__meta-flat');
    expect(container.querySelector('dd')).toHaveClass('applications-list__meta-value', 'applications-table-source');
});

test('omits the optional classes rather than emitting undefined', () => {
    const {container} = render(
        <EntryFieldList variant='tile'>
            <EntryField name='project' label='Project'>
                default
            </EntryField>
        </EntryFieldList>
    );

    expect(dl(container).className).toBe('entry-fields entry-fields--tile');
    expect(container.querySelector('dd').className).toBe('entry-fields__dd entry-fields__dd--project applications-tiles__field-value');
});
