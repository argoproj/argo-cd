import {act, render} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Layout} from '../shared/components/layout/layout';
import {Provider} from '../shared/context';
import {SIDEBAR_TOOLS_ID, Sidebar, useSidebarTarget} from './sidebar';

jest.mock('argo-ui', () => ({
    Tooltip: ({children}: {children: React.ReactElement}) => children
}));
jest.mock('argo-ui/v2', () => ({
    useData: () => [{Version: 'v1.0.0'}, false, null]
}));
jest.mock('../shared/services', () => ({
    services: {
        version: {version: () => Promise.resolve({Version: 'v1.0.0'})},
        viewPreferences: {updatePreferences: jest.fn()}
    }
}));

const Filters = () => <div data-testid='filters'>filters</div>;

// Mirrors how pages consume the hook: filters are portalled into the sidebar slot, and nothing is
// rendered inside the page body while the slot does not exist.
const Page = ({showFilters = true}: {showFilters?: boolean}) => {
    const sidebarTarget = useSidebarTarget();
    return <div className='page'>{showFilters && sidebarTarget && ReactDOM.createPortal(<Filters />, sidebarTarget)}</div>;
};

const createCtx = () => ({
    history: createMemoryHistory({initialEntries: ['/applications']}),
    navigation: {goto: jest.fn()},
    popup: {},
    notifications: {},
    baseHref: '/'
});

const renderWithLayout = (showFilters = true) => {
    const context = createCtx();
    const view = (nextShowFilters: boolean) => (
        <Provider value={context as any}>
            <Layout navItems={[]} pref={{hideSidebar: false, theme: 'light'} as any}>
                <Page showFilters={nextShowFilters} />
            </Layout>
        </Provider>
    );

    const result = render(view(showFilters));
    return {...result, rerenderWithLayout: (next: boolean) => result.rerender(view(next))};
};

test('portals the filters into the sidebar slot instead of the page body', () => {
    const {container} = renderWithLayout();

    const slot = container.querySelector(`#${SIDEBAR_TOOLS_ID}`);
    expect(slot).not.toBeNull();
    expect(slot.querySelector('[data-testid="filters"]')).not.toBeNull();
    expect(container.querySelector('.page [data-testid="filters"]')).toBeNull();
});

test('renders nothing and does not throw when there is no sidebar slot', () => {
    const {container} = render(<Page />);

    expect(container.querySelector(`#${SIDEBAR_TOOLS_ID}`)).toBeNull();
    expect(container.querySelector('[data-testid="filters"]')).toBeNull();
});

test('leaves the sidebar slot itself in place when the page stops rendering filters', () => {
    const {container, rerenderWithLayout} = renderWithLayout(true);

    const slot = container.querySelector(`#${SIDEBAR_TOOLS_ID}`);
    expect(slot.querySelector('[data-testid="filters"]')).not.toBeNull();

    rerenderWithLayout(false);

    // The slot belongs to the sidebar, so a page dropping its portal must not remove the container.
    expect(container.querySelector(`#${SIDEBAR_TOOLS_ID}`)).toBe(slot);
    expect(slot.childElementCount).toBe(0);
});

test('keeps a single stable sidebar slot while pages mount and unmount their filters', () => {
    const {container, rerenderWithLayout} = renderWithLayout();
    const slot = container.querySelector(`#${SIDEBAR_TOOLS_ID}`);

    for (let i = 0; i < 3; i++) {
        act(() => rerenderWithLayout(false));
        act(() => rerenderWithLayout(true));

        expect(container.querySelectorAll(`#${SIDEBAR_TOOLS_ID}`)).toHaveLength(1);
        expect(container.querySelector(`#${SIDEBAR_TOOLS_ID}`)).toBe(slot);
        expect(slot.querySelector('[data-testid="filters"]')).not.toBeNull();
    }
});

test('Sidebar renders the slot and hands it to the caller', () => {
    let received: HTMLDivElement | null = null;
    const {container} = render(
        <Provider value={createCtx() as any}>
            <Sidebar navItems={[]} pref={{hideSidebar: false} as any} onVersionClick={() => undefined} onToolsElement={el => (received = el)} />
        </Provider>
    );

    const slot = container.querySelector(`#${SIDEBAR_TOOLS_ID}`);
    expect(slot).not.toBeNull();
    expect(received).toBe(slot);
});
