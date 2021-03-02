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
