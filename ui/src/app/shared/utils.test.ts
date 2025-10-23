import {concatMaps} from './utils';

test('map concatenation', () => {
    const map1 = {
        a: '1',
        b: '2',
    };
    const map2 = {
        a: '9',
        c: '8',
    };
    const map3 = concatMaps(map1, map2);
    expect(map3).toEqual(new Map(Object.entries({a: '9', b: '2', c: '8'})));
});
