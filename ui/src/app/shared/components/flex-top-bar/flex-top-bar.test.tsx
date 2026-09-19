import {act, render} from '@testing-library/react';
import * as React from 'react';

import {FlexTopBar} from './flex-top-bar';

let resizeCallback: ResizeObserverCallback;
let resizeObserver: ResizeObserverMock;
let topBarHeight = 97.5;
let getBoundingClientRectSpy: jest.SpyInstance;
const originalResizeObserver = global.ResizeObserver;

class ResizeObserverMock implements ResizeObserver {
    constructor(callback: ResizeObserverCallback) {
        resizeCallback = callback;
        resizeObserver = this;
    }

    observe = jest.fn();
    unobserve = jest.fn();
    disconnect = jest.fn();
}

beforeAll(() => {
    const getBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;
    getBoundingClientRectSpy = jest.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
        if (this.classList.contains('flex-top-bar')) {
            return {height: topBarHeight} as DOMRect;
        }
        return getBoundingClientRect.call(this);
    });
});

beforeEach(() => {
    global.ResizeObserver = ResizeObserverMock;
    topBarHeight = 97.5;
});

afterAll(() => {
    global.ResizeObserver = originalResizeObserver;
    getBoundingClientRectSpy.mockRestore();
});

test('keeps the padder aligned with the rendered toolbar height', () => {
    const {container, unmount} = render(<FlexTopBar toolbar={{addAuth: false, tools: <div>Tools</div>}} />);
    const padder = container.querySelector<HTMLElement>('.flex-top-bar__padder');

    expect(padder).toHaveStyle({height: '97.5px'});
    expect(resizeObserver.observe).toHaveBeenCalledWith(container.querySelector('.flex-top-bar'));

    topBarHeight = 51;
    act(() => resizeCallback([], resizeObserver));

    expect(padder).toHaveStyle({height: '51px'});

    unmount();
    expect(resizeObserver.disconnect).toHaveBeenCalled();
});

test('falls back to window resize events when ResizeObserver is unavailable', () => {
    delete (global as {ResizeObserver?: typeof ResizeObserver}).ResizeObserver;

    const {container} = render(<FlexTopBar toolbar={{addAuth: false, tools: <div>Tools</div>}} />);
    const padder = container.querySelector<HTMLElement>('.flex-top-bar__padder');

    expect(padder).toHaveStyle({height: '97.5px'});

    topBarHeight = 51;
    act(() => {
        window.dispatchEvent(new Event('resize'));
    });

    expect(padder).toHaveStyle({height: '51px'});
});
