// How much of a large resource tree is drawn, and how that changes when the user asks for more.
//
// These decisions used to live inline among four hundred lines of graph building, where the values they
// juggle — the graph's budget, each parent's allowance, the defaults, the step, the ceiling — could only
// be exercised by loading a real application. Every fix to one of them risked another, and the reviews
// kept finding the seam. They are gathered here so the interactions can be tested on their own.

/** Nodes drawn before the budget starts refusing. Chosen to stay under a second of layout work. */
export const DEFAULT_VISIBLE_CAP = 200;
/** The most the budget will ever be raised to, however many times the user asks for more. */
export const MAX_VISIBLE_CAP = 1000;
/** Children drawn under one parent, so a single wide subtree cannot spend the whole budget. */
export const MAX_CHILDREN_PER_PARENT = 25;
/** Members previewed beneath a clustered kind. */
export const KIND_GROUP_PREVIEW = 5;
/** How much one click on an overflow control asks for. */
export const EXPAND_STEP = 50;

/** Counts the user has raised, keyed by the parent whose control they clicked. */
export type Allowances = {[key: string]: number};

/**
 * The allowance in effect for one parent: what the user raised it to, else the structural default.
 */
export function allowanceFor(allowances: Allowances, key: string, fallback: number): number {
    return allowances[key] ?? fallback;
}

/**
 * The allowance after one click. Takes the allowance that was in effect, never a count of what was
 * drawn: on a deep graph those differ, and starting from the drawn count would lower the allowance and
 * so reveal less than before the click.
 */
export function nextAllowance(previous: number | undefined, inEffect: number): number {
    return (previous ?? inEffect) + EXPAND_STEP;
}

/** The graph's budget after one click. Raised alongside the allowance, or the extra nodes are refused. */
export function nextCap(cap: number): number {
    return Math.min(cap + EXPAND_STEP, MAX_VISIBLE_CAP);
}

/**
 * Whether an overflow control has nothing left to offer. Both conditions matter: a parent's allowance
 * grows more slowly than the budget does, so judging this by the budget alone switched the control off
 * while the graph still had room.
 */
export function atCeiling(cap: number, drawn: number): boolean {
    return cap >= MAX_VISIBLE_CAP && drawn >= cap;
}

/**
 * Whether the user has actually filtered anything. The tree is always handed a filter callback, so the
 * callback's presence says nothing; the list of applied filters is the signal.
 */
export function filterIsActive(filters?: string[]): boolean {
    return (filters || []).length > 0;
}

/**
 * Two questions the budget used to answer with one number. Clustering is about how crowded the top level
 * is. Ordering is about whether the budget can bite at all, which it can through depth alone: 150
 * deployments with their replica sets and pods is fewer roots than the cap and far more nodes, and
 * ordering those by name drops the later chains unranked and hides filter matches inside them.
 */
export function rootStrategy(rootCount: number, nodeCount: number, cap: number, hasFilter: boolean, bucket?: string | null): {clusterKinds: boolean; rankRoots: boolean} {
    return {
        clusterKinds: rootCount > cap && !bucket,
        // A filter always forces ordering: without it a match cannot outrank whatever sorts first.
        rankRoots: hasFilter || !!bucket || rootCount > cap || nodeCount > cap
    };
}
