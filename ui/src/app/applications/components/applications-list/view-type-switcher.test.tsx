import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {ContextApis} from '../../../shared/context';
import {AppsListPreferences} from '../../../shared/services';
import {ViewTypeSwitcher} from './view-type-switcher';

const ctx = {navigation: {goto: jest.fn()}} as unknown as ContextApis;
const pref = (view: string) => ({view, page: 0, search: ''}) as unknown as AppsListPreferences & {page: number; search: string};

describe('ViewTypeSwitcher group toggle', () => {
    it('is not rendered unless the caller opts in (ApplicationSets page)', () => {
        render(<ViewTypeSwitcher pref={pref('tiles')} ctx={ctx} />);
        expect(screen.queryByTitle('Group by project')).toBeNull();
    });

    it('is not rendered in summary view', () => {
        render(<ViewTypeSwitcher pref={pref('summary')} ctx={ctx} groupByProject={{enabled: true, onToggle: jest.fn()}} />);
        expect(screen.queryByTitle('Group by project')).toBeNull();
    });

    it.each(['tiles', 'list'])('reflects state and toggles in %s view', view => {
        const onToggle = jest.fn();
        render(<ViewTypeSwitcher pref={pref(view)} ctx={ctx} groupByProject={{enabled: true, onToggle}} />);
        const toggle = screen.getByTitle('Group by project');
        expect(toggle.getAttribute('aria-pressed')).toBe('true');
        expect(toggle.className).toContain('selected');
        fireEvent.click(toggle);
        expect(onToggle).toHaveBeenCalledTimes(1);
    });
});
