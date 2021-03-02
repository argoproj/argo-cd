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

export enum NumKey {
    ZERO = 48,
    ONE = 49,
    TWO = 50,
    THREE = 51,
    FOUR = 52,
    FIVE = 53,
    SIX = 54,
    SEVEN = 55,
    EIGHT = 56,
    NINE = 57
}

export enum NumPadKey {
    ZERO = 96,
    ONE = 97,
    TWO = 98,
    THREE = 99,
    FOUR = 100,
    FIVE = 101,
    SIX = 102,
    SEVEN = 103,
    EIGHT = 104,
    NINE = 105
}

export type AnyNumKey = NumKey | NumPadKey;
export type AnyKeys = AnyNumKey | Key | (AnyNumKey | Key)[];

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

export type KeyAction = (keyCode?: number) => boolean;
export type KeyMap = {[key: number]: KeyAction};

export const useKeyListener = (): ((keys: AnyKeys, action: KeyAction) => void) => {
    const keyMap = {} as KeyMap;
    const handlePress = (e: KeyboardEvent) => {
        const action = keyMap[e.keyCode];
        if (action) {
            const prevent = action(e.keyCode);
            if (prevent) {
                e.preventDefault();
            }
        }
    };
    React.useEffect(() => {
        document.addEventListener('keydown', handlePress);
        return () => {
            document.removeEventListener('keydown', handlePress);
        };
    }, [keyMap]);
    return (keys, a) => {
        if (Array.isArray(keys)) {
            for (const key of keys) {
                keyMap[key as number] = a;
            }
        } else {
            keyMap[keys as number] = a;
        }
    };
};

export const NumKeyToNumber = (key: AnyNumKey): number => {
    if (key > 47 && key < 58) {
        return key - 48;
    } else if (key > 95 && key < 106) {
        return key - 96;
    }
    return -1;
};
