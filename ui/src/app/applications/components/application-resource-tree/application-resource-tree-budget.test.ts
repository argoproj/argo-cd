import {
    allowanceFor,
    atCeiling,
    DEFAULT_VISIBLE_CAP,
    EXPAND_STEP,
    filterIsActive,
    KIND_GROUP_PREVIEW,
    MAX_CHILDREN_PER_PARENT,
    MAX_VISIBLE_CAP,
    nextAllowance,
    nextCap,
    rootStrategy,
} from './application-resource-tree-budget';

describe('allowanceFor', () => {
    test('falls back to the structural default', () => {
        expect(allowanceFor({}, 'parent', MAX_CHILDREN_PER_PARENT)).toBe(MAX_CHILDREN_PER_PARENT);
    });

    test('prefers what the user raised it to', () => {
        expect(allowanceFor({parent: 75}, 'parent', MAX_CHILDREN_PER_PARENT)).toBe(75);
    });

    test('treats a raised count of zero as raised', () => {
        expect(allowanceFor({parent: 0}, 'parent', MAX_CHILDREN_PER_PARENT)).toBe(0);
    });
});

describe('nextAllowance', () => {
    test('a first click grows the default by one step', () => {
        expect(nextAllowance(undefined, DEFAULT_VISIBLE_CAP)).toBe(DEFAULT_VISIBLE_CAP + EXPAND_STEP);
    });

    test('later clicks grow what the user already asked for', () => {
        expect(nextAllowance(250, DEFAULT_VISIBLE_CAP)).toBe(300);
    });

    // The regression: a deep graph draws far fewer roots than the allowance admitted. Starting from the
    // drawn count made the control lower the allowance -- clicking "more" revealed less.
    test('never shrinks the allowance when little of it was drawn', () => {
        const drawnOnADeepGraph = 5;
        expect(nextAllowance(undefined, DEFAULT_VISIBLE_CAP)).toBeGreaterThan(drawnOnADeepGraph + EXPAND_STEP);
        expect(nextAllowance(undefined, DEFAULT_VISIBLE_CAP)).toBeGreaterThan(DEFAULT_VISIBLE_CAP);
    });

    test('grows a kind preview from its own small default', () => {
        expect(nextAllowance(undefined, KIND_GROUP_PREVIEW)).toBe(KIND_GROUP_PREVIEW + EXPAND_STEP);
    });
});

describe('nextCap', () => {
    test('grows by a step', () => {
        expect(nextCap(DEFAULT_VISIBLE_CAP)).toBe(DEFAULT_VISIBLE_CAP + EXPAND_STEP);
    });

    test('stops at the ceiling rather than promising what it cannot draw', () => {
        expect(nextCap(MAX_VISIBLE_CAP)).toBe(MAX_VISIBLE_CAP);
        expect(nextCap(MAX_VISIBLE_CAP - 10)).toBe(MAX_VISIBLE_CAP);
    });
});

describe('atCeiling', () => {
    test('room left below the ceiling', () => {
        expect(atCeiling(DEFAULT_VISIBLE_CAP, 10)).toBe(false);
    });

    // The regression: a parent's allowance grows more slowly than the budget, so a graph at its cap can
    // still have room. Judging by the cap alone switched the control off with slots to spare.
    test('at the ceiling but not yet full, so the control stays useful', () => {
        expect(atCeiling(MAX_VISIBLE_CAP, 825)).toBe(false);
    });

    test('at the ceiling and full', () => {
        expect(atCeiling(MAX_VISIBLE_CAP, MAX_VISIBLE_CAP)).toBe(true);
    });

    test('budget spent but the ceiling not reached, so more can still be asked for', () => {
        expect(atCeiling(DEFAULT_VISIBLE_CAP, DEFAULT_VISIBLE_CAP)).toBe(false);
    });
});

describe('filterIsActive', () => {
    // The regression: the tree is always handed a filter callback, so presence proved nothing and every
    // application was reordered as though filtered.
    test('nothing applied', () => {
        expect(filterIsActive(undefined)).toBe(false);
        expect(filterIsActive([])).toBe(false);
    });

    test('something applied', () => {
        expect(filterIsActive(['kind:Pod'])).toBe(true);
    });
});

describe('rootStrategy', () => {
    const CAP = DEFAULT_VISIBLE_CAP;

    test('ranks roots when the budget can only bite through depth', () => {
        // 150 deployments with their replica sets and pods: fewer roots than the cap, far more nodes.
        expect(rootStrategy(150, 750, CAP, false)).toEqual({clusterKinds: false, rankRoots: true});
    });

    test('leaves a small application in its familiar order', () => {
        expect(rootStrategy(10, 40, CAP, false)).toEqual({clusterKinds: false, rankRoots: false});
    });

    test('ranks whenever a filter is active, however small the application', () => {
        expect(rootStrategy(10, 40, CAP, true)).toEqual({clusterKinds: false, rankRoots: true});
    });

    test('clusters kinds only when the top level itself is crowded', () => {
        expect(rootStrategy(1500, 4000, CAP, false)).toEqual({clusterKinds: true, rankRoots: true});
    });

    test('drilling into a kind shows it flat, and ranked', () => {
        expect(rootStrategy(1500, 4000, CAP, false, 'ConfigMap')).toEqual({clusterKinds: false, rankRoots: true});
    });
});

// The two signals the component derives from its props, checked together the way it uses them: the
// caller-bug class that the earlier tests could not catch, because they took an already-derived boolean.
describe('the signals a caller has to derive', () => {
    test('an unfiltered application keeps its order however the callback is supplied', () => {
        const alwaysSuppliedCallback = () => true;
        expect(typeof alwaysSuppliedCallback).toBe('function');
        // What the component must not do is read that callback's presence as an active filter.
        expect(rootStrategy(10, 40, DEFAULT_VISIBLE_CAP, filterIsActive([]))).toEqual({clusterKinds: false, rankRoots: false});
    });

    test('the same application with a filter applied is ranked', () => {
        expect(rootStrategy(10, 40, DEFAULT_VISIBLE_CAP, filterIsActive(['name:cm-3999']))).toEqual({clusterKinds: false, rankRoots: true});
    });

    test('one click on a deep graph raises the admission window instead of lowering it', () => {
        const rootAllowance = allowanceFor({}, 'app', DEFAULT_VISIBLE_CAP);
        const drawn = 5; // a deep graph spent the budget on descendants
        const raised = nextAllowance(undefined, rootAllowance);
        expect(raised).toBeGreaterThan(rootAllowance);
        expect(raised).not.toBe(drawn + EXPAND_STEP);
    });
});
