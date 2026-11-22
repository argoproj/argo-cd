import * as React from 'react';
import {cleanup, render, screen} from '@testing-library/react';
import type {ApplicationSet} from '../../../shared/models';
import {VIRTUAL_THRESHOLD} from './virtual-scroll';

jest.mock('argo-ui', () => {
    const actual = jest.requireActual('argo-ui');
    return {
        ...actual,
        DataLoader: ({children}: {children: (data: unknown) => React.ReactNode}) => children({appList: {favoritesAppList: []}}),
        Tooltip: ({children}: {children?: React.ReactNode}) => <>{children}</>
    };
});

jest.mock('argo-ui/v2', () => ({
    Key: {RIGHT: 'ArrowRight', LEFT: 'ArrowLeft', DOWN: 'ArrowDown', UP: 'ArrowUp', ENTER: 'Enter', ESCAPE: 'Escape'},
    NumKey: {},
    NumPadKey: {},
    NumKeyToNumber: () => 0,
    KeybindingContext: React.createContext({registerKeybinding: jest.fn()}),
    useNav: () => [-1, jest.fn(), jest.fn()]
}));

jest.mock('../../../shared/context', () => ({
    Consumer: ({children}: {children: (ctx: {navigation: {goto: jest.Mock}}) => React.ReactNode}) => children({navigation: {goto: jest.fn()}}),
    Context: React.createContext({navigation: {goto: jest.fn()}}),
    AuthSettingsCtx: React.createContext({})
}));

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {
            getPreferences: () => ({
                subscribe: (observer: {next: (v: unknown) => void}) => {
                    observer.next({appList: {favoritesAppList: []}});
                    return {unsubscribe: jest.fn()};
                }
            }),
            updatePreferences: jest.fn()
        }
    }
}));

jest.mock('./appset-tile', () => ({
    AppSetTile: ({appSet}: {appSet: ApplicationSet}) => <div data-testid={`appset-tile-${appSet.metadata.name}`}>{appSet.metadata.name}</div>
}));

jest.mock('./appset-table-row', () => ({
    AppSetTableRow: ({appSet}: {appSet: ApplicationSet}) => <div data-testid={`appset-row-${appSet.metadata.name}`}>{appSet.metadata.name}</div>
}));

const mockVirtualizedTilesGrid = jest.fn(({applications}: {applications: ApplicationSet[]}) => (
    <div data-testid='virtualized-tiles-grid' data-count={applications.length} />
));

jest.mock('./applications-tiles', () => ({
    VirtualizedTilesGrid: (props: {applications: ApplicationSet[]}) => mockVirtualizedTilesGrid(props)
}));

jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: (size: {width: number}) => React.ReactNode}) => children({width: 1200})
}));

jest.mock('react-virtualized/dist/commonjs/WindowScroller', () => ({
    __esModule: true,
    default: ({children}: {children: (props: object) => React.ReactNode}) => children({height: 600, isScrolling: false, onChildScroll: jest.fn(), scrollTop: 0})
}));

jest.mock('react-virtualized/dist/commonjs/List', () => ({
    __esModule: true,
    default: ({rowCount, rowRenderer}: {rowCount: number; rowRenderer: (props: {index: number; key: string; style: React.CSSProperties}) => React.ReactNode}) => (
        <div data-testid='virtualized-table-list' data-count={rowCount}>
            {Array.from({length: Math.min(rowCount, 3)}, (_, index) => rowRenderer({index, key: String(index), style: {}}))}
        </div>
    )
}));

import {ApplicationSetTable, ApplicationSetTiles} from './application-sets-list';

const makeAppSets = (count: number): ApplicationSet[] =>
    Array.from({length: count}, (_, i) => ({
        kind: 'ApplicationSet',
        metadata: {name: `appset-${i}`, namespace: 'argocd'},
        spec: {generators: [], template: {metadata: {}, spec: {project: 'default', source: {}, destination: {}}}}
    })) as ApplicationSet[];

describe('ApplicationSetTiles virtual scrolling', () => {
    afterEach(() => {
        cleanup();
        jest.clearAllMocks();
    });

    it('uses VirtualizedTilesGrid when Items per page is all and count is above the threshold', () => {
        const appSets = makeAppSets(VIRTUAL_THRESHOLD + 1);
        render(<ApplicationSetTiles appSets={appSets} useVirtualScrolling={true} />);

        expect(screen.getByTestId('virtualized-tiles-grid')).toHaveAttribute('data-count', String(appSets.length));
        expect(screen.queryByTestId('appset-tile-appset-0')).toBeNull();
        expect(mockVirtualizedTilesGrid).toHaveBeenCalled();
    });

    it('renders all tiles without VirtualizedTilesGrid below the threshold', () => {
        const appSets = makeAppSets(VIRTUAL_THRESHOLD);
        render(<ApplicationSetTiles appSets={appSets} useVirtualScrolling={true} />);

        expect(screen.queryByTestId('virtualized-tiles-grid')).toBeNull();
        expect(screen.getByTestId('appset-tile-appset-0')).toBeInTheDocument();
        expect(screen.getByTestId(`appset-tile-appset-${VIRTUAL_THRESHOLD - 1}`)).toBeInTheDocument();
    });

    it('renders all tiles without VirtualizedTilesGrid when Items per page is not all', () => {
        const appSets = makeAppSets(VIRTUAL_THRESHOLD + 1);
        render(<ApplicationSetTiles appSets={appSets} useVirtualScrolling={false} />);

        expect(screen.queryByTestId('virtualized-tiles-grid')).toBeNull();
        expect(screen.getByTestId('appset-tile-appset-0')).toBeInTheDocument();
    });
});

describe('ApplicationSetTable virtual scrolling', () => {
    afterEach(() => {
        cleanup();
        jest.clearAllMocks();
    });

    it('uses a virtualized List when Items per page is all and count is above the threshold', () => {
        const appSets = makeAppSets(VIRTUAL_THRESHOLD + 1);
        render(<ApplicationSetTable appSets={appSets} useVirtualScrolling={true} />);

        expect(screen.getByTestId('virtualized-table-list')).toHaveAttribute('data-count', String(appSets.length));
        // Mock List only mounts a few rows (overscan stand-in); not every AppSet row.
        expect(screen.getByTestId('appset-row-appset-0')).toBeInTheDocument();
        expect(screen.queryByTestId(`appset-row-appset-${VIRTUAL_THRESHOLD}`)).toBeNull();
    });

    it('renders all rows without a virtualized List below the threshold', () => {
        const appSets = makeAppSets(VIRTUAL_THRESHOLD);
        render(<ApplicationSetTable appSets={appSets} useVirtualScrolling={true} />);

        expect(screen.queryByTestId('virtualized-table-list')).toBeNull();
        expect(screen.getByTestId('appset-row-appset-0')).toBeInTheDocument();
        expect(screen.getByTestId(`appset-row-appset-${VIRTUAL_THRESHOLD - 1}`)).toBeInTheDocument();
    });
});
