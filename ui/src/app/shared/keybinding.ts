import * as React from 'react';

export enum Key {
    ENTER = 13,
    ESCAPE = 27,
    LEFT = 37,
    UP = 38,
    RIGHT = 39,
    DOWN = 40,
    SLASH = 191
}

// useNav adds simple stateful navigation to your component
// Returns:
//   - pos: indicates current position
//   - nav: fxn that accepts an integer that represents number to increment/decrement pos
//   - reset: fxn that sets current position to -1
// Accepts:
//   - upperBound: maximum value that pos can grow to
//   - init: optional initial value for pos

export const useNav = (upperBound: number, init?: number): [number, (n: number) => boolean, () => void] => {
    const [pos, setPos] = React.useState(init || -1);
    const isInBounds = (p: number): boolean => p < upperBound && p > -1;

    const nav = (val: number): boolean => {
        const newPos = pos + val;
        return isInBounds(newPos) ? setPos(newPos) === null : false;
    };

    const reset = () => {
        setPos(-1);
    };

    return [pos, nav, reset];
};

export const useKeyPress = (key: Key | Key[], action: () => boolean) => {
    React.useEffect(() => {
        const handlePress = (e: KeyboardEvent) => {
            const keys = Array.isArray(key) ? key : [key];
            for (const k of keys) {
                if (e.keyCode === k) {
                    if (action()) {
                        e.preventDefault();
                    }
                }
            }
        };
        document.addEventListener('keydown', handlePress);
        return () => {
            document.removeEventListener('keydown', handlePress);
        };
    });
};
