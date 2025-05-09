declare module 'history' {
    export type Action = 'PUSH' | 'POP' | 'REPLACE';

    export interface Location<S = LocationState> {
        pathname: string;
        search: string;
        state: S;
        hash: string;
        key?: string;
    }

    export type LocationState = any; // Or a more specific type if you know it for your app
    export type LocationDescriptorObject<S = LocationState> = {
        pathname?: string;
        search?: string;
        state?: S;
        hash?: string;
        key?: string;
    };
    export type LocationDescriptor<S = LocationState> = string | LocationDescriptorObject<S>;

    export interface History<S = LocationState> {
        length: number;
        action: Action;
        location: Location<S>;
        push(path: LocationDescriptor<S>, state?: S): void;
        push(location: Location<S>): void; // Overload for pushing a location object directly
        replace(path: LocationDescriptor<S>, state?: S): void;
        replace(location: Location<S>): void; // Overload for replacing with a location object
        go(n: number): void;
        goBack(): void;
        goForward(): void;
        block(prompt?: boolean | string | ((location: Location<S>, action: Action) => string)): () => void;
        listen(listener: (location: Location<S>, action: Action) => void): () => void;
        createHref(location: LocationDescriptorObject<S>): string;
    }

    export interface BrowserHistoryBuildOptions {
        basename?: string;
        forceRefresh?: boolean;
        getUserConfirmation?: (message: string, callback: (ok: boolean) => void) => void;
        keyLength?: number;
    }
    export function createBrowserHistory<S = LocationState>(options?: BrowserHistoryBuildOptions): History<S>;

    // Add other history creators if needed (createMemoryHistory, createHashHistory)
} 